package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestTokenManagerGenerateAndParse(t *testing.T) {
	manager := NewTokenManager("test-secret")
	token, err := manager.GenerateAccessToken(User{ID: 10, Email: "jane@example.com"})
	if err != nil {
		t.Fatalf("expected token generation to succeed: %v", err)
	}

	claims, err := manager.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("expected token parse to succeed: %v", err)
	}
	if claims.Subject != "10" || claims.Email != "jane@example.com" {
		t.Fatalf("expected user claims, got %#v", claims)
	}
}

func TestAuthFlow(t *testing.T) {
	server := newTestServer(t)

	registerBody := `{"name":"Jane Doe","email":"jane@example.com","password":"secret123"}`
	registerResponse := doJSON(t, server, http.MethodPost, "/auth/register", "", registerBody, http.StatusCreated)
	accessToken := stringField(t, registerResponse, "access_token")
	refreshToken := stringField(t, registerResponse, "refresh_token")

	doJSON(t, server, http.MethodGet, "/me", accessToken, "", http.StatusOK)

	loginBody := `{"email":"jane@example.com","password":"secret123"}`
	doJSON(t, server, http.MethodPost, "/auth/login", "", loginBody, http.StatusOK)

	refreshBody := `{"refresh_token":"` + refreshToken + `"}`
	refreshResponse := doJSON(t, server, http.MethodPost, "/auth/refresh", "", refreshBody, http.StatusOK)
	newRefreshToken := stringField(t, refreshResponse, "refresh_token")

	doJSON(t, server, http.MethodPost, "/auth/refresh", "", refreshBody, http.StatusUnauthorized)
	doJSON(t, server, http.MethodPost, "/auth/logout", "", `{"refresh_token":"`+newRefreshToken+`"}`, http.StatusNoContent)
}

func TestRegisterValidation(t *testing.T) {
	server := newTestServer(t)

	body := `{"name":"","email":"invalid","password":"123"}`
	doJSON(t, server, http.MethodPost, "/auth/register", "", body, http.StatusBadRequest)
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("expected database open to succeed: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	store := NewStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatalf("expected migrate to succeed: %v", err)
	}

	handler := NewHandler(store, NewTokenManager("test-secret"))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func doJSON(t *testing.T, handler http.Handler, method string, path string, accessToken string, body string, wantStatus int) map[string]any {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("expected status %d, got %d with body %s", wantStatus, response.Code, response.Body.String())
	}
	if response.Body.Len() == 0 {
		return map[string]any{}
	}

	var data map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatalf("expected json response: %v", err)
	}

	return data
}

func stringField(t *testing.T, data map[string]any, key string) string {
	t.Helper()

	value, ok := data[key].(string)
	if !ok || value == "" {
		t.Fatalf("expected string field %q, got %#v", key, data[key])
	}

	return value
}
