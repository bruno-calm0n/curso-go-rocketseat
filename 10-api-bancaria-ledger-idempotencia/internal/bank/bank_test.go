package bank

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLedgerAndIdempotency(t *testing.T) {
	server := newTestServer(t)

	account := requestJSON(t, server, http.MethodPost, "/accounts", "account-1", `{"owner_name":"Jane Doe"}`, http.StatusCreated)
	accountID := int64(account["id"].(float64))

	depositPath := "/accounts/" + strconv.FormatInt(accountID, 10) + "/deposit"
	firstDeposit := requestJSON(t, server, http.MethodPost, depositPath, "deposit-1", `{"amount":100}`, http.StatusOK)
	secondDeposit := requestJSON(t, server, http.MethodPost, depositPath, "deposit-1", `{"amount":100}`, http.StatusOK)
	if firstDeposit["balance"] != secondDeposit["balance"] {
		t.Fatalf("expected repeated idempotent response, got %#v and %#v", firstDeposit, secondDeposit)
	}

	balance := requestJSON(t, server, http.MethodGet, "/accounts/"+strconv.FormatInt(accountID, 10)+"/balance", "", "", http.StatusOK)
	if balance["balance"].(float64) != 100 {
		t.Fatalf("expected balance 100, got %#v", balance)
	}

	ledger := requestArray(t, server, "/accounts/"+strconv.FormatInt(accountID, 10)+"/ledger")
	if len(ledger) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(ledger))
	}
}

func TestTransferAndInsufficientFunds(t *testing.T) {
	server := newTestServer(t)

	from := requestJSON(t, server, http.MethodPost, "/accounts", "account-from", `{"owner_name":"Origem"}`, http.StatusCreated)
	to := requestJSON(t, server, http.MethodPost, "/accounts", "account-to", `{"owner_name":"Destino"}`, http.StatusCreated)
	fromID := int64(from["id"].(float64))
	toID := int64(to["id"].(float64))

	requestJSON(t, server, http.MethodPost, "/accounts/"+strconv.FormatInt(fromID, 10)+"/deposit", "deposit-from", `{"amount":80}`, http.StatusOK)
	requestJSON(t, server, http.MethodPost, "/transfers", "transfer-1", `{"from_account_id":`+strconv.FormatInt(fromID, 10)+`,"to_account_id":`+strconv.FormatInt(toID, 10)+`,"amount":30}`, http.StatusOK)
	requestJSON(t, server, http.MethodPost, "/accounts/"+strconv.FormatInt(fromID, 10)+"/withdraw", "withdraw-too-much", `{"amount":1000}`, http.StatusBadRequest)

	fromBalance := requestJSON(t, server, http.MethodGet, "/accounts/"+strconv.FormatInt(fromID, 10)+"/balance", "", "", http.StatusOK)
	toBalance := requestJSON(t, server, http.MethodGet, "/accounts/"+strconv.FormatInt(toID, 10)+"/balance", "", "", http.StatusOK)
	if fromBalance["balance"].(float64) != 50 || toBalance["balance"].(float64) != 30 {
		t.Fatalf("expected transfer balances 50 and 30, got %#v and %#v", fromBalance, toBalance)
	}
}

func TestIdempotencyKeyRequired(t *testing.T) {
	server := newTestServer(t)

	requestJSON(t, server, http.MethodPost, "/accounts", "", `{"owner_name":"Jane Doe"}`, http.StatusBadRequest)
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "bank.db"))
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

	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, key string, body string, wantStatus int) map[string]any {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
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

func requestArray(t *testing.T, handler http.Handler, path string) []any {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", response.Code, response.Body.String())
	}

	var data []any
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatalf("expected json array: %v", err)
	}

	return data
}
