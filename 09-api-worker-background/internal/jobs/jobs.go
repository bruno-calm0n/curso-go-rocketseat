package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrJobNotFound = errors.New("job not found")

type Job struct {
	ID        int64     `json:"id"`
	Payload   string    `json:"payload"`
	Status    string    `json:"status"`
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

// NewStore recebe a conexao pronta para facilitar testes.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Migrate cria a tabela de jobs usada pela API e pelo worker.
func (s *Store) Migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			payload TEXT NOT NULL,
			status TEXT NOT NULL,
			result TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			attempts INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
	`

	_, err := s.db.Exec(query)
	return err
}

// CreateJob salva um job pendente para processamento futuro.
func (s *Store) CreateJob(payload string) (Job, error) {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		"INSERT INTO jobs (payload, status, created_at, updated_at) VALUES (?, ?, ?, ?)",
		payload,
		"pending",
		now,
		now,
	)
	if err != nil {
		return Job{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Job{}, err
	}

	return Job{ID: id, Payload: payload, Status: "pending", CreatedAt: now, UpdatedAt: now}, nil
}

// FindAll lista jobs do mais recente para o mais antigo.
func (s *Store) FindAll() ([]Job, error) {
	rows, err := s.db.Query("SELECT id, payload, status, result, error, attempts, created_at, updated_at FROM jobs ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]Job, 0)
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.ID, &job.Payload, &job.Status, &job.Result, &job.Error, &job.Attempts, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

// FindByID retorna o estado atual de um job.
func (s *Store) FindByID(id int64) (Job, error) {
	var job Job
	err := s.db.QueryRow(
		"SELECT id, payload, status, result, error, attempts, created_at, updated_at FROM jobs WHERE id = ?",
		id,
	).Scan(&job.ID, &job.Payload, &job.Status, &job.Result, &job.Error, &job.Attempts, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

// ClaimNextPending marca um job como processing antes de executar.
func (s *Store) ClaimNextPending() (Job, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Job{}, false, err
	}
	defer tx.Rollback()

	var job Job
	err = tx.QueryRow(
		"SELECT id, payload, attempts FROM jobs WHERE status = ? ORDER BY id ASC LIMIT 1",
		"pending",
	).Scan(&job.ID, &job.Payload, &job.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}

	now := time.Now().UTC()
	_, err = tx.Exec(
		"UPDATE jobs SET status = ?, attempts = attempts + 1, updated_at = ? WHERE id = ?",
		"processing",
		now,
		job.ID,
	)
	if err != nil {
		return Job{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, false, err
	}

	job.Status = "processing"
	job.Attempts++
	job.UpdatedAt = now
	return job, true, nil
}

// CompleteJob grava o resultado final do processamento.
func (s *Store) CompleteJob(id int64, result string) error {
	_, err := s.db.Exec(
		"UPDATE jobs SET status = ?, result = ?, error = '', updated_at = ? WHERE id = ?",
		"done",
		result,
		time.Now().UTC(),
		id,
	)
	return err
}

// FailJob registra falha sem perder o historico do job.
func (s *Store) FailJob(id int64, message string) error {
	_, err := s.db.Exec(
		"UPDATE jobs SET status = ?, error = ?, updated_at = ? WHERE id = ?",
		"failed",
		message,
		time.Now().UTC(),
		id,
	)
	return err
}

type Worker struct {
	store    *Store
	interval time.Duration
}

// NewWorker recebe o store e o intervalo de busca por jobs.
func NewWorker(store *Store, interval time.Duration) *Worker {
	return &Worker{store: store, interval: interval}
}

// Start executa o loop ate o contexto ser cancelado.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		w.ProcessNext()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ProcessNext processa um unico job pendente.
func (w *Worker) ProcessNext() (bool, error) {
	job, found, err := w.store.ClaimNextPending()
	if err != nil || !found {
		return found, err
	}

	result, err := processPayload(job.Payload)
	if err != nil {
		return true, w.store.FailJob(job.ID, err.Error())
	}

	return true, w.store.CompleteJob(job.ID, result)
}

// processPayload representa o trabalho pesado executado fora da requisicao.
func processPayload(payload string) (string, error) {
	cleanPayload := strings.TrimSpace(payload)
	if cleanPayload == "" {
		return "", errors.New("empty payload")
	}

	return "processed: " + strings.ToUpper(cleanPayload), nil
}

type Handler struct {
	store *Store
}

// NewHandler injeta o store usado pelos endpoints.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes registra a criacao e consulta dos jobs.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /jobs", h.Create)
	mux.HandleFunc("GET /jobs", h.FindAll)
	mux.HandleFunc("GET /jobs/{id}", h.FindByID)
}

// Create enfileira um job e responde rapido com status pending.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	payload := strings.TrimSpace(request.Payload)
	if payload == "" {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}

	job, err := h.store.CreateJob(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	writeJSON(w, http.StatusAccepted, job)
}

// FindAll lista jobs para acompanhar a fila.
func (h *Handler) FindAll(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.store.FindAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch jobs")
		return
	}

	writeJSON(w, http.StatusOK, jobs)
}

// FindByID consulta o status de um job especifico.
func (h *Handler) FindByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	job, err := h.store.FindByID(id)
	if errors.Is(err, ErrJobNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch job")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

type createJobRequest struct {
	Payload string `json:"payload"`
}

func parseID(w http.ResponseWriter, value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}

	return id, true
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}
