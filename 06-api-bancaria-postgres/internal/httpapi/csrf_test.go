package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFManagerGenerateAndValidate(t *testing.T) {
	// Token gerado pela propria chave deve validar com sucesso.
	manager := NewCSRFManager("test-secret")

	token, err := manager.Generate()
	if err != nil {
		t.Fatalf("expected token generation to succeed: %v", err)
	}

	if !manager.Valid(token) {
		t.Fatal("expected generated token to be valid")
	}
}

func TestCSRFManagerRejectsInvalidToken(t *testing.T) {
	manager := NewCSRFManager("test-secret")

	if manager.Valid("") {
		t.Fatal("expected empty token to be invalid")
	}

	if manager.Valid("invalid-token") {
		t.Fatal("expected malformed token to be invalid")
	}
}

func TestCSRFMiddleware(t *testing.T) {
	manager := NewCSRFManager("test-secret")
	token, err := manager.Generate()
	if err != nil {
		t.Fatalf("expected token generation to succeed: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Com token valido, a requisicao chega ao proximo handler.
	request := httptest.NewRequest(http.MethodPost, "/conta", nil)
	request.Header.Set(csrfHeader, token)
	response := httptest.NewRecorder()

	manager.Middleware(next).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", response.Code)
	}
}

func TestCSRFMiddlewareRejectsMissingToken(t *testing.T) {
	manager := NewCSRFManager("test-secret")

	request := httptest.NewRequest(http.MethodPost, "/conta", nil)
	response := httptest.NewRecorder()

	manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", response.Code)
	}
}
