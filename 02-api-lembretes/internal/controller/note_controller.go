package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"api-lembretes/internal/domain"
	"api-lembretes/internal/repository"
)

type NoteController struct {
	repository repository.NoteRepository
}

// NewNoteController injeta a dependencia de armazenamento no controller.
func NewNoteController(repository repository.NoteRepository) *NoteController {
	return &NoteController{repository: repository}
}

// RegisterRoutes centraliza as rotas HTTP de lembretes.
func (c *NoteController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /notes", c.Create)
	mux.HandleFunc("GET /notes", c.FindAll)
	mux.HandleFunc("GET /notes/{id}", c.FindByID)
	mux.HandleFunc("PUT /notes/{id}", c.Update)
	mux.HandleFunc("DELETE /notes/{id}", c.Delete)
}

// Create valida o JSON recebido e cria um novo lembrete.
func (c *NoteController) Create(w http.ResponseWriter, r *http.Request) {
	var input noteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	note, ok := input.toNote()
	if !ok {
		writeError(w, http.StatusBadRequest, "title e content sao obrigatorios")
		return
	}

	createdNote, err := c.repository.Create(note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao criar lembrete")
		return
	}

	writeJSON(w, http.StatusCreated, createdNote)
}

// FindAll retorna todos os lembretes cadastrados.
func (c *NoteController) FindAll(w http.ResponseWriter, r *http.Request) {
	notes, err := c.repository.FindAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao listar lembretes")
		return
	}

	writeJSON(w, http.StatusOK, notes)
}

// FindByID busca um lembrete pelo id recebido na URL.
func (c *NoteController) FindByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	note, err := c.repository.FindByID(id)
	if errors.Is(err, repository.ErrNoteNotFound) {
		writeError(w, http.StatusNotFound, "lembrete nao encontrado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao buscar lembrete")
		return
	}

	writeJSON(w, http.StatusOK, note)
}

// Update valida os dados e atualiza o lembrete existente.
func (c *NoteController) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var input noteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	note, ok := input.toNote()
	if !ok {
		writeError(w, http.StatusBadRequest, "title e content sao obrigatorios")
		return
	}
	note.ID = id

	updatedNote, err := c.repository.Update(note)
	if errors.Is(err, repository.ErrNoteNotFound) {
		writeError(w, http.StatusNotFound, "lembrete nao encontrado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao atualizar lembrete")
		return
	}

	writeJSON(w, http.StatusOK, updatedNote)
}

// Delete remove um lembrete pelo id informado.
func (c *NoteController) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	err := c.repository.Delete(id)
	if errors.Is(err, repository.ErrNoteNotFound) {
		writeError(w, http.StatusNotFound, "lembrete nao encontrado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao deletar lembrete")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type noteInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// toNote limpa e valida os campos antes de montar o dominio.
func (input noteInput) toNote() (domain.Note, bool) {
	title := strings.TrimSpace(input.Title)
	content := strings.TrimSpace(input.Content)
	if title == "" || content == "" {
		return domain.Note{}, false
	}

	return domain.Note{
		Title:   title,
		Content: content,
	}, true
}

// parseID garante que o id da rota seja um inteiro positivo.
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id invalido")
		return 0, false
	}

	return id, true
}

// writeJSON padroniza todas as respostas JSON da API.
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError devolve erros sempre no mesmo formato.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}
