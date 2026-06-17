package tasks

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrTaskNotFound = errors.New("task not found")

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Filters struct {
	Status   string
	Priority string
	Page     int
	Limit    int
	Sort     string
}

type Store struct {
	db *sql.DB
}

// NewStore recebe a conexao pronta para manter o acesso a dados desacoplado.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Migrate cria a tabela de tarefas se ela ainda nao existir.
func (s *Store) Migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			status TEXT NOT NULL,
			priority TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
	`

	_, err := s.db.Exec(query)
	return err
}

// Create salva uma tarefa nova com status inicial pendente.
func (s *Store) Create(task Task) (Task, error) {
	now := time.Now().UTC()
	task.Status = "pending"
	task.CreatedAt = now
	task.UpdatedAt = now

	result, err := s.db.Exec(
		"INSERT INTO tasks (title, description, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		task.Title,
		task.Description,
		task.Status,
		task.Priority,
		task.CreatedAt,
		task.UpdatedAt,
	)
	if err != nil {
		return Task{}, err
	}

	task.ID, err = result.LastInsertId()
	if err != nil {
		return Task{}, err
	}

	return task, nil
}

// FindAll aplica filtros e paginacao usando argumentos seguros.
func (s *Store) FindAll(filters Filters) ([]Task, int, error) {
	where, args := filtersWhere(filters)

	countQuery := "SELECT COUNT(*) FROM tasks" + where
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := "created_at DESC"
	if filters.Sort == "oldest" {
		orderBy = "created_at ASC"
	}

	offset := (filters.Page - 1) * filters.Limit
	query := "SELECT id, title, description, status, priority, created_at, updated_at FROM tasks" + where + " ORDER BY " + orderBy + " LIMIT ? OFFSET ?"
	args = append(args, filters.Limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// FindByID retorna uma tarefa ou erro de dominio quando nao existe.
func (s *Store) FindByID(id int64) (Task, error) {
	var task Task
	err := s.db.QueryRow(
		"SELECT id, title, description, status, priority, created_at, updated_at FROM tasks WHERE id = ?",
		id,
	).Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, err
	}

	return task, nil
}

// Update valida existencia antes de alterar os campos editaveis.
func (s *Store) Update(id int64, updates Task) (Task, error) {
	current, err := s.FindByID(id)
	if err != nil {
		return Task{}, err
	}

	current.Title = updates.Title
	current.Description = updates.Description
	current.Status = updates.Status
	current.Priority = updates.Priority
	current.UpdatedAt = time.Now().UTC()

	_, err = s.db.Exec(
		"UPDATE tasks SET title = ?, description = ?, status = ?, priority = ?, updated_at = ? WHERE id = ?",
		current.Title,
		current.Description,
		current.Status,
		current.Priority,
		current.UpdatedAt,
		id,
	)
	if err != nil {
		return Task{}, err
	}

	return current, nil
}

// Delete remove a tarefa e retorna o registro deletado.
func (s *Store) Delete(id int64) (Task, error) {
	task, err := s.FindByID(id)
	if err != nil {
		return Task{}, err
	}

	if _, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id); err != nil {
		return Task{}, err
	}

	return task, nil
}

func filtersWhere(filters Filters) (string, []any) {
	conditions := make([]string, 0)
	args := make([]any, 0)

	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	}
	if filters.Priority != "" {
		conditions = append(conditions, "priority = ?")
		args = append(args, filters.Priority)
	}
	if len(conditions) == 0 {
		return "", args
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

type Handler struct {
	store *Store
}

// NewHandler injeta o store usado pela camada HTTP.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes registra o CRUD de tarefas.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tasks", h.Create)
	mux.HandleFunc("GET /api/tasks", h.FindAll)
	mux.HandleFunc("GET /api/tasks/{id}", h.FindByID)
	mux.HandleFunc("PUT /api/tasks/{id}", h.Update)
	mux.HandleFunc("DELETE /api/tasks/{id}", h.Delete)
}

// Create valida entrada e cria tarefa.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input taskInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	task, ok := input.toCreateTask()
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid task data")
		return
	}

	createdTask, err := h.store.Create(task)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	writeJSON(w, http.StatusCreated, createdTask)
}

// FindAll retorna tarefas com filtros, pagina e total.
func (h *Handler) FindAll(w http.ResponseWriter, r *http.Request) {
	filters := parseFilters(r)
	tasks, total, err := h.store.FindAll(filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch tasks")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  tasks,
		"page":  filters.Page,
		"limit": filters.Limit,
		"total": total,
	})
}

// FindByID busca uma tarefa especifica.
func (h *Handler) FindByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	task, err := h.store.FindByID(id)
	if errors.Is(err, ErrTaskNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch task")
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// Update altera uma tarefa existente.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	var input taskInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	task, ok := input.toUpdateTask()
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid task data")
		return
	}

	updatedTask, err := h.store.Update(id, task)
	if errors.Is(err, ErrTaskNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update task")
		return
	}

	writeJSON(w, http.StatusOK, updatedTask)
}

// Delete remove uma tarefa existente.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	deletedTask, err := h.store.Delete(id)
	if errors.Is(err, ErrTaskNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}

	writeJSON(w, http.StatusOK, deletedTask)
}

type taskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
}

func (input taskInput) toCreateTask() (Task, bool) {
	input.Status = "pending"
	return input.toUpdateTask()
}

// toUpdateTask normaliza e valida os campos permitidos.
func (input taskInput) toUpdateTask() (Task, bool) {
	task := Task{
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		Status:      strings.ToLower(strings.TrimSpace(input.Status)),
		Priority:    strings.ToLower(strings.TrimSpace(input.Priority)),
	}

	if task.Title == "" || task.Description == "" {
		return Task{}, false
	}
	if !validStatus(task.Status) || !validPriority(task.Priority) {
		return Task{}, false
	}

	return task, true
}

func parseFilters(r *http.Request) Filters {
	query := r.URL.Query()
	page := parseInt(query.Get("page"), 1)
	limit := parseInt(query.Get("limit"), 10)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	status := strings.ToLower(strings.TrimSpace(query.Get("status")))
	if !validStatus(status) {
		status = ""
	}
	priority := strings.ToLower(strings.TrimSpace(query.Get("priority")))
	if !validPriority(priority) {
		priority = ""
	}

	sort := strings.ToLower(strings.TrimSpace(query.Get("sort")))
	if sort != "oldest" {
		sort = "newest"
	}

	return Filters{Status: status, Priority: priority, Page: page, Limit: limit, Sort: sort}
}

func parseID(w http.ResponseWriter, value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}

	return id, true
}

func parseInt(value string, fallback int) int {
	number, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return number
}

func validStatus(status string) bool {
	return status == "" || status == "pending" || status == "doing" || status == "done"
}

func validPriority(priority string) bool {
	return priority == "" || priority == "low" || priority == "medium" || priority == "high"
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}
