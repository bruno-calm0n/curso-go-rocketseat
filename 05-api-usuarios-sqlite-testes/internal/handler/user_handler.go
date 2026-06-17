package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"api-usuarios-sqlite-testes/internal/domain"
	"api-usuarios-sqlite-testes/internal/store"
)

type UserHandler struct {
	store store.UserStore
}

// NewUserHandler recebe a interface de armazenamento usada pelos handlers.
func NewUserHandler(store store.UserStore) *UserHandler {
	return &UserHandler{store: store}
}

// RegisterRoutes registra todos os endpoints de usuarios.
func (h *UserHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/users", h.Create)
	mux.HandleFunc("GET /api/users", h.FindAll)
	mux.HandleFunc("GET /api/users/{id}", h.FindByID)
	mux.HandleFunc("PUT /api/users/{id}", h.Update)
	mux.HandleFunc("DELETE /api/users/{id}", h.Delete)
}

// Create valida o corpo da requisicao e cria um usuario.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input userInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	user, ok := input.toUser()
	if !ok {
		writeError(w, http.StatusBadRequest, "first_name, last_name and biography are required with valid lengths")
		return
	}

	createdUser, err := h.store.Insert(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save user")
		return
	}

	writeJSON(w, http.StatusCreated, createdUser)
}

// FindAll devolve todos os usuarios cadastrados.
func (h *UserHandler) FindAll(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.FindAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch users")
		return
	}

	writeJSON(w, http.StatusOK, users)
}

// FindByID busca um usuario pelo id da URL.
func (h *UserHandler) FindByID(w http.ResponseWriter, r *http.Request) {
	user, err := h.store.FindByID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch user")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// Update substitui os dados principais de um usuario existente.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input userInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	userUpdates, ok := input.toUser()
	if !ok {
		writeError(w, http.StatusBadRequest, "first_name, last_name and biography are required with valid lengths")
		return
	}

	updatedUser, err := h.store.Update(r.PathValue("id"), userUpdates)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}
	if updatedUser == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSON(w, http.StatusOK, updatedUser)
}

// Delete remove um usuario e retorna o registro removido.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	deletedUser, err := h.store.Delete(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}
	if deletedUser == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSON(w, http.StatusOK, deletedUser)
}

type userInput struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Biography string `json:"biography"`
}

// toUser limpa os textos e aplica as regras de tamanho do desafio.
func (input userInput) toUser() (domain.User, bool) {
	firstName := strings.TrimSpace(input.FirstName)
	lastName := strings.TrimSpace(input.LastName)
	biography := strings.TrimSpace(input.Biography)

	if !validText(firstName, 2, 20) || !validText(lastName, 2, 20) || !validText(biography, 20, 450) {
		return domain.User{}, false
	}

	return domain.User{
		FirstName: firstName,
		LastName:  lastName,
		Biography: biography,
	}, true
}

// validText conta runes para validar tambem textos com acentos.
func validText(value string, min int, max int) bool {
	size := len([]rune(value))
	return size >= min && size <= max
}

// writeJSON padroniza as respostas de sucesso da API.
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError padroniza as respostas de erro da API.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}
