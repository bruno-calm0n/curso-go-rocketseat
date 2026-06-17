package jobs

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestWorkerProcessesJob(t *testing.T) {
	store := newTestStore(t)
	worker := NewWorker(store, time.Hour)

	job, err := store.CreateJob("enviar email")
	if err != nil {
		t.Fatalf("expected job creation to succeed: %v", err)
	}

	processed, err := worker.ProcessNext()
	if err != nil {
		t.Fatalf("expected worker to succeed: %v", err)
	}
	if !processed {
		t.Fatal("expected one job to be processed")
	}

	foundJob, err := store.FindByID(job.ID)
	if err != nil {
		t.Fatalf("expected job lookup to succeed: %v", err)
	}
	if foundJob.Status != "done" || foundJob.Result != "processed: ENVIAR EMAIL" {
		t.Fatalf("expected processed job, got %#v", foundJob)
	}
}

func TestJobHTTPFlow(t *testing.T) {
	store := newTestStore(t)
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	created := requestJSON(t, mux, http.MethodPost, "/jobs", `{"payload":"gerar relatorio"}`, http.StatusAccepted)
	id := int64(created["id"].(float64))

	requestStatus(t, mux, http.MethodGet, "/jobs", "", http.StatusOK)
	requestJSON(t, mux, http.MethodGet, "/jobs/"+strconv.FormatInt(id, 10), "", http.StatusOK)
	requestJSON(t, mux, http.MethodPost, "/jobs", `{"payload":""}`, http.StatusBadRequest)
	requestJSON(t, mux, http.MethodGet, "/jobs/abc", "", http.StatusBadRequest)
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "jobs.db"))
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

	return store
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

func requestStatus(t *testing.T, handler http.Handler, method string, path string, body string, wantStatus int) {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != wantStatus {
		t.Fatalf("expected status %d, got %d with body %s", wantStatus, response.Code, response.Body.String())
	}
}
