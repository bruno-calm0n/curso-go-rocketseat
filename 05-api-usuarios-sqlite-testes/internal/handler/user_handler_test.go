package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"api-usuarios-sqlite-testes/internal/domain"
	"api-usuarios-sqlite-testes/internal/store"

	_ "modernc.org/sqlite"
)

func TestCreateUser(t *testing.T) {
	// Sobe a API em memoria de teste com SQLite temporario.
	server := newTestServer(t)

	response, body := requestJSON(t, http.MethodPost, server.URL+"/api/users", validUserJSON())
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d with body %s", response.StatusCode, body)
	}

	var user domain.User
	decodeJSON(t, body, &user)
	if user.ID == "" || user.FirstName != "Jane" {
		t.Fatalf("expected created user with id, got %#v", user)
	}
}

func TestCreateUserValidationErrors(t *testing.T) {
	server := newTestServer(t)

	// Casos de entrada invalida devem sempre retornar erro JSON.
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{"first_name":`},
		{name: "missing biography", body: `{"first_name":"Jane","last_name":"Doe"}`},
		{name: "short first name", body: `{"first_name":"J","last_name":"Doe","biography":"Tendo diversao estudando Go todos os dias."}`},
		{name: "long last name", body: `{"first_name":"Jane","last_name":"SobrenomeGrandeDemais","biography":"Tendo diversao estudando Go todos os dias."}`},
		{name: "short biography", body: `{"first_name":"Jane","last_name":"Doe","biography":"curta"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, body := requestJSON(t, http.MethodPost, server.URL+"/api/users", test.body)
			defer response.Body.Close()

			assertStatus(t, response, body, http.StatusBadRequest)
			assertErrorResponse(t, body)
		})
	}
}

func TestFindAllUsers(t *testing.T) {
	server := newTestServer(t)

	createUser(t, server, `{"first_name":"Jane","last_name":"Doe","biography":"Tendo diversao estudando Go todos os dias."}`)
	createUser(t, server, `{"first_name":"John","last_name":"Doe","biography":"Outra biografia valida para os testes."}`)

	response, body := requestJSON(t, http.MethodGet, server.URL+"/api/users", "")
	defer response.Body.Close()

	assertStatus(t, response, body, http.StatusOK)

	var users []domain.User
	decodeJSON(t, body, &users)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestFindUserByID(t *testing.T) {
	server := newTestServer(t)
	createdUser := createUser(t, server, validUserJSON())

	response, body := requestJSON(t, http.MethodGet, server.URL+"/api/users/"+createdUser.ID, "")
	defer response.Body.Close()

	assertStatus(t, response, body, http.StatusOK)

	var foundUser domain.User
	decodeJSON(t, body, &foundUser)
	if foundUser.ID != createdUser.ID {
		t.Fatalf("expected user id %s, got %s", createdUser.ID, foundUser.ID)
	}
}

func TestFindUserByIDNotFound(t *testing.T) {
	server := newTestServer(t)

	response, body := requestJSON(t, http.MethodGet, server.URL+"/api/users/unknown-id", "")
	defer response.Body.Close()

	assertStatus(t, response, body, http.StatusNotFound)
	assertErrorResponse(t, body)
}

func TestUpdateUser(t *testing.T) {
	server := newTestServer(t)
	createdUser := createUser(t, server, validUserJSON())

	updateBody := `{"first_name":"Maria","last_name":"Silva","biography":"Uma biografia valida para atualizar o usuario."}`
	response, body := requestJSON(t, http.MethodPut, server.URL+"/api/users/"+createdUser.ID, updateBody)
	defer response.Body.Close()

	assertStatus(t, response, body, http.StatusOK)

	var updatedUser domain.User
	decodeJSON(t, body, &updatedUser)
	if updatedUser.ID != createdUser.ID || updatedUser.FirstName != "Maria" || updatedUser.LastName != "Silva" {
		t.Fatalf("expected updated user, got %#v", updatedUser)
	}
}

func TestUpdateUserErrors(t *testing.T) {
	server := newTestServer(t)

	tests := []struct {
		name       string
		id         string
		body       string
		wantStatus int
	}{
		{name: "not found", id: "unknown-id", body: validUserJSON(), wantStatus: http.StatusNotFound},
		{name: "invalid json", id: "unknown-id", body: `{"first_name":`, wantStatus: http.StatusBadRequest},
		{name: "invalid body", id: "unknown-id", body: `{"first_name":"J","last_name":"Doe","biography":"Tendo diversao estudando Go todos os dias."}`, wantStatus: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, body := requestJSON(t, http.MethodPut, server.URL+"/api/users/"+test.id, test.body)
			defer response.Body.Close()

			assertStatus(t, response, body, test.wantStatus)
			assertErrorResponse(t, body)
		})
	}
}

func TestDeleteUser(t *testing.T) {
	server := newTestServer(t)
	createdUser := createUser(t, server, validUserJSON())

	response, body := requestJSON(t, http.MethodDelete, server.URL+"/api/users/"+createdUser.ID, "")
	defer response.Body.Close()

	assertStatus(t, response, body, http.StatusOK)

	var deletedUser domain.User
	decodeJSON(t, body, &deletedUser)
	if deletedUser.ID != createdUser.ID {
		t.Fatalf("expected deleted user id %s, got %s", createdUser.ID, deletedUser.ID)
	}

	response, body = requestJSON(t, http.MethodGet, server.URL+"/api/users/"+createdUser.ID, "")
	defer response.Body.Close()

	assertStatus(t, response, body, http.StatusNotFound)
}

func TestDeleteUserNotFound(t *testing.T) {
	server := newTestServer(t)

	response, body := requestJSON(t, http.MethodDelete, server.URL+"/api/users/unknown-id", "")
	defer response.Body.Close()

	assertStatus(t, response, body, http.StatusNotFound)
	assertErrorResponse(t, body)
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Cada teste usa um banco isolado para nao compartilhar estado.
	dbPath := filepath.Join(t.TempDir(), "users.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("expected database open to succeed: %v", err)
	}

	userStore := store.NewSQLiteUserStore(db)
	if err := userStore.Migrate(); err != nil {
		t.Fatalf("expected migrate to succeed: %v", err)
	}

	userHandler := NewUserHandler(userStore)
	mux := http.NewServeMux()
	userHandler.RegisterRoutes(mux)

	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		server.Close()
		db.Close()
	})

	return server
}

// requestJSON envia uma chamada HTTP e devolve status + corpo para as assercoes.
func requestJSON(t *testing.T, method string, url string, body string) (*http.Response, []byte) {
	t.Helper()

	var requestBody *bytes.Reader
	if body == "" {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = bytes.NewReader([]byte(body))
	}

	request, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		t.Fatalf("expected request creation to succeed: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("expected request to succeed: %v", err)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("expected response read to succeed: %v", err)
	}

	return response, responseBody
}

// createUser reaproveita o fluxo real de POST para preparar dados dos testes.
func createUser(t *testing.T, server *httptest.Server, body string) domain.User {
	t.Helper()

	response, responseBody := requestJSON(t, http.MethodPost, server.URL+"/api/users", body)
	defer response.Body.Close()

	assertStatus(t, response, responseBody, http.StatusCreated)

	var user domain.User
	decodeJSON(t, responseBody, &user)
	return user
}

func assertStatus(t *testing.T, response *http.Response, body []byte, want int) {
	t.Helper()

	if response.StatusCode != want {
		t.Fatalf("expected status %d, got %d with body %s", want, response.StatusCode, body)
	}
}

func assertErrorResponse(t *testing.T, body []byte) {
	t.Helper()

	var errorResponse map[string]string
	decodeJSON(t, body, &errorResponse)
	if errorResponse["error"] == "" {
		t.Fatalf("expected error message, got %s", body)
	}
}

func decodeJSON(t *testing.T, body []byte, target any) {
	t.Helper()

	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("expected valid json response: %v, body: %s", err, body)
	}
}

func validUserJSON() string {
	return `{"first_name":"Jane","last_name":"Doe","biography":"Tendo diversao estudando Go todos os dias."}`
}
