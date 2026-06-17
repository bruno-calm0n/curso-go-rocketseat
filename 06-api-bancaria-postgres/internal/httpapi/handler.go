package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"api-bancaria-postgres/internal/domain"
	"api-bancaria-postgres/internal/repository"
)

type AccountHandler struct {
	repository repository.AccountRepository
}

// NewAccountHandler recebe a interface de repositorio usada pela API.
func NewAccountHandler(repository repository.AccountRepository) *AccountHandler {
	return &AccountHandler{repository: repository}
}

// CreateAccount valida o payload e cria conta PF ou PJ.
func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var request createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	input, ok := request.toInput()
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid account data")
		return
	}

	account, err := h.repository.Create(r.Context(), input)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, account)
}

// GetBalance retorna o saldo atual da conta informada na URL.
func (h *AccountHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	ref, ok := parseAccountRef(w, r.PathValue("id"))
	if !ok {
		return
	}

	balance, err := h.repository.GetBalance(r.Context(), ref)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, balance)
}

// Deposit soma um valor positivo ao saldo da conta.
func (h *AccountHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	ref, ok := parseAccountRef(w, r.PathValue("id"))
	if !ok {
		return
	}

	amount, ok := readAmount(w, r)
	if !ok {
		return
	}

	balance, err := h.repository.Deposit(r.Context(), ref, amount)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, balance)
}

// Withdraw subtrai um valor quando houver saldo suficiente.
func (h *AccountHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	ref, ok := parseAccountRef(w, r.PathValue("id"))
	if !ok {
		return
	}

	amount, ok := readAmount(w, r)
	if !ok {
		return
	}

	balance, err := h.repository.Withdraw(r.Context(), ref, amount)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, balance)
}

// Transfer move saldo entre duas contas dentro de uma transacao.
func (h *AccountHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	var request transferRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	from, err := domain.ParseAccountID(request.FromID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source account id")
		return
	}

	to, err := domain.ParseAccountID(request.ToID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination account id")
		return
	}

	if request.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	result, err := h.repository.Transfer(r.Context(), from, to, request.Amount)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CloseAccount remove a conta e retorna os dados deletados.
func (h *AccountHandler) CloseAccount(w http.ResponseWriter, r *http.Request) {
	ref, ok := parseAccountRef(w, r.PathValue("id"))
	if !ok {
		return
	}

	account, err := h.repository.Close(r.Context(), ref)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, account)
}

type createAccountRequest struct {
	Type             string  `json:"tipo"`
	Income           float64 `json:"renda_mensal"`
	Revenue          float64 `json:"faturamento"`
	Age              int     `json:"idade"`
	FullName         string  `json:"nome_completo"`
	TradeName        string  `json:"nome_fantasia"`
	Phone            string  `json:"celular"`
	Email            string  `json:"email"`
	CorporateEmail   string  `json:"email_corporativo"`
	Category         string  `json:"categoria"`
	AvailableBalance float64 `json:"saldo"`
}

// toInput normaliza e valida os campos antes de chamar o repositorio.
func (r createAccountRequest) toInput() (domain.CreateAccountInput, bool) {
	accountType := domain.AccountType(strings.ToLower(strings.TrimSpace(r.Type)))
	input := domain.CreateAccountInput{
		Type:             accountType,
		Income:           r.Income,
		Revenue:          r.Revenue,
		Age:              r.Age,
		FullName:         strings.TrimSpace(r.FullName),
		TradeName:        strings.TrimSpace(r.TradeName),
		Phone:            strings.TrimSpace(r.Phone),
		Email:            strings.TrimSpace(r.Email),
		CorporateEmail:   strings.TrimSpace(r.CorporateEmail),
		Category:         strings.TrimSpace(r.Category),
		AvailableBalance: r.AvailableBalance,
	}

	if input.Age <= 0 || input.AvailableBalance < 0 || input.Phone == "" || input.Category == "" {
		return domain.CreateAccountInput{}, false
	}

	switch accountType {
	case domain.AccountTypePF:
		return input, input.Income >= 0 && input.FullName != "" && input.Email != ""
	case domain.AccountTypePJ:
		return input, input.Revenue >= 0 && input.TradeName != "" && input.CorporateEmail != ""
	default:
		return domain.CreateAccountInput{}, false
	}
}

type amountRequest struct {
	Amount float64 `json:"valor"`
}

type transferRequest struct {
	FromID string  `json:"origem_id"`
	ToID   string  `json:"destino_id"`
	Amount float64 `json:"valor"`
}

// readAmount valida o JSON das operacoes de deposito e saque.
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

// parseAccountRef valida o id publico recebido pela rota.
func parseAccountRef(w http.ResponseWriter, value string) (domain.AccountRef, bool) {
	ref, err := domain.ParseAccountID(value)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return domain.AccountRef{}, false
	}

	return ref, true
}

// handleDomainError traduz erros de dominio para status HTTP.
func handleDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrAccountNotFound):
		writeError(w, http.StatusNotFound, "account not found")
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeError(w, http.StatusBadRequest, "insufficient funds")
	case errors.Is(err, domain.ErrInvalidAccountID), errors.Is(err, domain.ErrInvalidAccountType):
		writeError(w, http.StatusBadRequest, "invalid account")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
