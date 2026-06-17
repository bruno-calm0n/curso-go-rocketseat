package tasks

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

func TestTaskCRUD(t *testing.T) {
	server := newTestServer(t)

	created := requestJSON(t, server, http.MethodPost, "/api/tasks", validTaskJSON("Estudar Go", "high"), http.StatusCreated)
	id := int64(created["id"].(float64))

	requestJSON(t, server, http.MethodGet, "/api/tasks/"+itoa(id), "", http.StatusOK)

	updateBody := `{"title":"Estudar testes","description":"Criar testes HTTP e de store","status":"doing","priority":"medium"}`
	updated := requestJSON(t, server, http.MethodPut, "/api/tasks/"+itoa(id), updateBody, http.StatusOK)
	if updated["status"] != "doing" {
		t.Fatalf("expected updated status, got %#v", updated)
	}

	requestJSON(t, server, http.MethodDelete, "/api/tasks/"+itoa(id), "", http.StatusOK)
	requestJSON(t, server, http.MethodGet, "/api/tasks/"+itoa(id), "", http.StatusNotFound)
}

func TestTaskFiltersAndPagination(t *testing.T) {
	server := newTestServer(t)

	requestJSON(t, server, http.MethodPost, "/api/tasks", validTaskJSON("Alta", "high"), http.StatusCreated)
	requestJSON(t, server, http.MethodPost, "/api/tasks", validTaskJSON("Media", "medium"), http.StatusCreated)
	requestJSON(t, server, http.MethodPost, "/api/tasks", validTaskJSON("Outra alta", "high"), http.StatusCreated)

	response := requestJSON(t, server, http.MethodGet, "/api/tasks?priority=high&page=1&limit=1", "", http.StatusOK)
	if response["total"].(float64) != 2 {
		t.Fatalf("expected total 2, got %#v", response["total"])
	}
	data := response["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected page with 1 task, got %d", len(data))
	}
}

func TestTaskValidation(t *testing.T) {
	server := newTestServer(t)

	body := `{"title":"","description":"","priority":"urgent"}`
	requestJSON(t, server, http.MethodPost, "/api/tasks", body, http.StatusBadRequest)
	requestJSON(t, server, http.MethodGet, "/api/tasks/abc", "", http.StatusBadRequest)
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "tasks.db"))
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

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body string, wantStatus int) map[string]any {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
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

func validTaskJSON(title string, priority string) string {
	return `{"title":"` + title + `","description":"Descricao valida da tarefa","priority":"` + priority + `"}`
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
