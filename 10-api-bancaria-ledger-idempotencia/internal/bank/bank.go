package bank

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ErrAccountNotFound   = errors.New("account not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidAmount     = errors.New("invalid amount")
)

type Account struct {
	ID        int64     `json:"id"`
	OwnerName string    `json:"owner_name"`
	CreatedAt time.Time `json:"created_at"`
}

type Balance struct {
	AccountID int64   `json:"account_id"`
	Balance   float64 `json:"balance"`
}

type LedgerEntry struct {
	ID               int64     `json:"id"`
	AccountID        int64     `json:"account_id"`
	Type             string    `json:"type"`
	Amount           float64   `json:"amount"`
	RelatedAccountID *int64    `json:"related_account_id,omitempty"`
	IdempotencyKey   string    `json:"idempotency_key"`
	CreatedAt        time.Time `json:"created_at"`
}

type TransferResult struct {
	From Balance `json:"from"`
	To   Balance `json:"to"`
}

type Store struct {
	db *sql.DB
}

// NewStore recebe a conexao pronta por injecao de dependencia.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Migrate cria as tabelas de contas, ledger e idempotencia.
func (s *Store) Migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_name TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS ledger_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			amount NUMERIC NOT NULL CHECK (amount > 0),
			related_account_id INTEGER,
			idempotency_key TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (account_id) REFERENCES accounts(id)
		);

		CREATE TABLE IF NOT EXISTS idempotency_records (
			key TEXT PRIMARY KEY,
			status_code INTEGER NOT NULL,
			response_body TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
	`

	_, err := s.db.Exec(query)
	return err
}

// CreateAccount cria uma conta sem saldo inicial.
func (s *Store) CreateAccount(ownerName string) (Account, error) {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		"INSERT INTO accounts (owner_name, created_at) VALUES (?, ?)",
		ownerName,
		now,
	)
	if err != nil {
		return Account{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Account{}, err
	}

	return Account{ID: id, OwnerName: ownerName, CreatedAt: now}, nil
}

// GetBalance calcula o saldo a partir do ledger imutavel.
func (s *Store) GetBalance(accountID int64) (Balance, error) {
	if err := s.ensureAccountExists(accountID); err != nil {
		return Balance{}, err
	}

	balance, err := s.balance(accountID)
	if err != nil {
		return Balance{}, err
	}

	return Balance{AccountID: accountID, Balance: balance}, nil
}

// Deposit registra entrada positiva no ledger.
func (s *Store) Deposit(accountID int64, amount float64, key string) (Balance, error) {
	if amount <= 0 {
		return Balance{}, ErrInvalidAmount
	}
	if err := s.ensureAccountExists(accountID); err != nil {
		return Balance{}, err
	}

	if err := s.insertEntry(accountID, "deposit", amount, nil, key); err != nil {
		return Balance{}, err
	}

	return s.GetBalance(accountID)
}

// Withdraw registra saida somente se houver saldo suficiente.
func (s *Store) Withdraw(accountID int64, amount float64, key string) (Balance, error) {
	if amount <= 0 {
		return Balance{}, ErrInvalidAmount
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Balance{}, err
	}
	defer tx.Rollback()

	balance, err := balanceTx(tx, accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return Balance{}, ErrAccountNotFound
	}
	if err != nil {
		return Balance{}, err
	}
	if balance < amount {
		return Balance{}, ErrInsufficientFunds
	}

	if err := insertEntryTx(tx, accountID, "withdraw", amount, nil, key); err != nil {
		return Balance{}, err
	}
	if err := tx.Commit(); err != nil {
		return Balance{}, err
	}

	return s.GetBalance(accountID)
}

// Transfer cria dois lancamentos: saida na origem e entrada no destino.
func (s *Store) Transfer(fromID int64, toID int64, amount float64, key string) (TransferResult, error) {
	if amount <= 0 || fromID == toID {
		return TransferResult{}, ErrInvalidAmount
	}

	tx, err := s.db.Begin()
	if err != nil {
		return TransferResult{}, err
	}
	defer tx.Rollback()

	fromBalance, err := balanceTx(tx, fromID)
	if errors.Is(err, sql.ErrNoRows) {
		return TransferResult{}, ErrAccountNotFound
	}
	if err != nil {
		return TransferResult{}, err
	}
	if _, err := balanceTx(tx, toID); errors.Is(err, sql.ErrNoRows) {
		return TransferResult{}, ErrAccountNotFound
	} else if err != nil {
		return TransferResult{}, err
	}
	if fromBalance < amount {
		return TransferResult{}, ErrInsufficientFunds
	}

	if err := insertEntryTx(tx, fromID, "transfer_out", amount, &toID, key); err != nil {
		return TransferResult{}, err
	}
	if err := insertEntryTx(tx, toID, "transfer_in", amount, &fromID, key); err != nil {
		return TransferResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return TransferResult{}, err
	}

	from, err := s.GetBalance(fromID)
	if err != nil {
		return TransferResult{}, err
	}
	to, err := s.GetBalance(toID)
	if err != nil {
		return TransferResult{}, err
	}

	return TransferResult{From: from, To: to}, nil
}

// Ledger lista todos os lancamentos da conta.
func (s *Store) Ledger(accountID int64) ([]LedgerEntry, error) {
	if err := s.ensureAccountExists(accountID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		"SELECT id, account_id, type, amount, related_account_id, idempotency_key, created_at FROM ledger_entries WHERE account_id = ? ORDER BY id ASC",
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]LedgerEntry, 0)
	for rows.Next() {
		var entry LedgerEntry
		var related sql.NullInt64
		if err := rows.Scan(&entry.ID, &entry.AccountID, &entry.Type, &entry.Amount, &related, &entry.IdempotencyKey, &entry.CreatedAt); err != nil {
			return nil, err
		}
		if related.Valid {
			entry.RelatedAccountID = &related.Int64
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// FindIdempotency retorna a resposta salva para uma chave repetida.
func (s *Store) FindIdempotency(key string) (int, string, bool, error) {
	var statusCode int
	var responseBody string
	err := s.db.QueryRow(
		"SELECT status_code, response_body FROM idempotency_records WHERE key = ?",
		key,
	).Scan(&statusCode, &responseBody)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}

	return statusCode, responseBody, true, nil
}

// SaveIdempotency guarda a resposta da primeira execucao da chave.
func (s *Store) SaveIdempotency(key string, statusCode int, responseBody string) error {
	_, err := s.db.Exec(
		"INSERT INTO idempotency_records (key, status_code, response_body, created_at) VALUES (?, ?, ?, ?)",
		key,
		statusCode,
		responseBody,
		time.Now().UTC(),
	)
	return err
}

func (s *Store) ensureAccountExists(accountID int64) error {
	var exists int
	err := s.db.QueryRow("SELECT 1 FROM accounts WHERE id = ?", accountID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountNotFound
	}
	return err
}

func (s *Store) balance(accountID int64) (float64, error) {
	return balanceQuery(s.db.QueryRow, accountID)
}

func (s *Store) insertEntry(accountID int64, entryType string, amount float64, relatedID *int64, key string) error {
	return insertEntryQuery(s.db.Exec, accountID, entryType, amount, relatedID, key)
}

func balanceTx(tx *sql.Tx, accountID int64) (float64, error) {
	var exists int
	if err := tx.QueryRow("SELECT 1 FROM accounts WHERE id = ?", accountID).Scan(&exists); err != nil {
		return 0, err
	}

	return balanceQuery(tx.QueryRow, accountID)
}

func insertEntryTx(tx *sql.Tx, accountID int64, entryType string, amount float64, relatedID *int64, key string) error {
	return insertEntryQuery(tx.Exec, accountID, entryType, amount, relatedID, key)
}

func balanceQuery(queryRow func(string, ...any) *sql.Row, accountID int64) (float64, error) {
	var balance float64
	err := queryRow(`
		SELECT COALESCE(SUM(
			CASE
				WHEN type IN ('deposit', 'transfer_in') THEN amount
				WHEN type IN ('withdraw', 'transfer_out') THEN -amount
				ELSE 0
			END
		), 0)
		FROM ledger_entries
		WHERE account_id = ?
	`, accountID).Scan(&balance)

	return balance, err
}

func insertEntryQuery(exec func(string, ...any) (sql.Result, error), accountID int64, entryType string, amount float64, relatedID *int64, key string) error {
	_, err := exec(
		"INSERT INTO ledger_entries (account_id, type, amount, related_account_id, idempotency_key, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		accountID,
		entryType,
		amount,
		relatedID,
		key,
		time.Now().UTC(),
	)
	return err
}

type Handler struct {
	store *Store
}

// NewHandler injeta o store nos handlers.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes registra contas, saldo, ledger e operacoes financeiras.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /accounts", h.CreateAccount)
	mux.HandleFunc("GET /accounts/{id}/balance", h.GetBalance)
	mux.HandleFunc("GET /accounts/{id}/ledger", h.GetLedger)
	mux.HandleFunc("POST /accounts/{id}/deposit", h.Deposit)
	mux.HandleFunc("POST /accounts/{id}/withdraw", h.Withdraw)
	mux.HandleFunc("POST /transfers", h.Transfer)
}

// CreateAccount cria uma conta nova.
func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	h.withIdempotency(w, r, func() (int, any, error) {
		var request createAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return http.StatusBadRequest, errorBody("invalid json"), nil
		}

		ownerName := strings.TrimSpace(request.OwnerName)
		if ownerName == "" {
			return http.StatusBadRequest, errorBody("owner_name is required"), nil
		}

		account, err := h.store.CreateAccount(ownerName)
		return http.StatusCreated, account, err
	})
}

// GetBalance mostra o saldo calculado pelo ledger.
func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	balance, err := h.store.GetBalance(id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, balance)
}

// GetLedger lista o historico imutavel da conta.
func (h *Handler) GetLedger(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	entries, err := h.store.Ledger(id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

// Deposit registra um lancamento de entrada.
func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	h.withIdempotency(w, r, func() (int, any, error) {
		amount, ok := readAmount(w, r)
		if !ok {
			return 0, nil, nil
		}

		balance, err := h.store.Deposit(id, amount, r.Header.Get("Idempotency-Key"))
		return http.StatusOK, balance, err
	})
}

// Withdraw registra uma saida se houver saldo.
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	h.withIdempotency(w, r, func() (int, any, error) {
		amount, ok := readAmount(w, r)
		if !ok {
			return 0, nil, nil
		}

		balance, err := h.store.Withdraw(id, amount, r.Header.Get("Idempotency-Key"))
		return http.StatusOK, balance, err
	})
}

// Transfer move dinheiro entre contas usando dois lancamentos.
func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	h.withIdempotency(w, r, func() (int, any, error) {
		var request transferRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return http.StatusBadRequest, errorBody("invalid json"), nil
		}

		result, err := h.store.Transfer(request.FromAccountID, request.ToAccountID, request.Amount, r.Header.Get("Idempotency-Key"))
		return http.StatusOK, result, err
	})
}

// withIdempotency reaproveita a resposta da primeira requisicao com a mesma chave.
func (h *Handler) withIdempotency(w http.ResponseWriter, r *http.Request, fn func() (int, any, error)) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	statusCode, responseBody, found, err := h.store.FindIdempotency(key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check idempotency")
		return
	}
	if found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(responseBody))
		return
	}

	statusCode, payload, err := fn()
	if err != nil {
		handleDomainError(w, err)
		return
	}
	if payload == nil {
		return
	}

	rawBody, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}
	if err := h.store.SaveIdempotency(key, statusCode, string(rawBody)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save idempotency")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(rawBody)
}

type createAccountRequest struct {
	OwnerName string `json:"owner_name"`
}

type amountRequest struct {
	Amount float64 `json:"amount"`
}

type transferRequest struct {
	FromAccountID int64   `json:"from_account_id"`
	ToAccountID   int64   `json:"to_account_id"`
	Amount        float64 `json:"amount"`
}

func readAmount(w http.ResponseWriter, r *http.Request) (float64, bool) {
	var request amountRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return 0, false
	}
	if request.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be greater than zero")
		return 0, false
	}

	return request.Amount, true
}

func parseID(w http.ResponseWriter, value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}

	return id, true
}

func handleDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrAccountNotFound):
		writeError(w, http.StatusNotFound, "account not found")
	case errors.Is(err, ErrInsufficientFunds):
		writeError(w, http.StatusBadRequest, "insufficient funds")
	case errors.Is(err, ErrInvalidAmount):
		writeError(w, http.StatusBadRequest, "invalid amount")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, errorBody(message))
}

func errorBody(message string) map[string]string {
	return map[string]string{"error": message}
}
