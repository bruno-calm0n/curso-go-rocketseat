package smoke

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestBankingAPISmoke(t *testing.T) {
	// Smoke test roda apenas quando a API e o PostgreSQL locais estiverem ativos.
	if os.Getenv("SMOKE_TEST") != "1" {
		t.Skip("set SMOKE_TEST=1 to run smoke tests against a running server")
	}

	baseURL := getenv("SMOKE_BASE_URL", "http://localhost:8080")
	client := &http.Client{Timeout: 5 * time.Second}

	token := getCSRFToken(t, client, baseURL)

	pf := postJSON(t, client, baseURL+"/conta", token, map[string]any{
		"tipo":          "pf",
		"renda_mensal":  5000.00,
		"idade":         30,
		"nome_completo": "Jane Doe",
		"celular":       "11999999999",
		"email":         fmt.Sprintf("jane.%d@example.com", time.Now().UnixNano()),
		"categoria":     "cliente",
		"saldo":         100.00,
	}, http.StatusCreated)

	pj := postJSON(t, client, baseURL+"/conta", token, map[string]any{
		"tipo":              "pj",
		"faturamento":       250000.00,
		"idade":             5,
		"nome_fantasia":     "Acme LTDA",
		"celular":           "11888888888",
		"email_corporativo": fmt.Sprintf("financeiro.%d@acme.test", time.Now().UnixNano()),
		"categoria":         "empresa",
		"saldo":             25.00,
	}, http.StatusCreated)

	pfID := stringValue(t, pf, "id")
	pjID := stringValue(t, pj, "id")

	getJSON(t, client, baseURL+"/conta/"+pfID+"/saldo", http.StatusOK)
	postJSON(t, client, baseURL+"/conta/"+pfID+"/deposito", token, map[string]any{"valor": 50.00}, http.StatusOK)
	postJSON(t, client, baseURL+"/conta/"+pfID+"/saque", token, map[string]any{"valor": 25.00}, http.StatusOK)
	postJSON(t, client, baseURL+"/conta/transferencia", token, map[string]any{
		"origem_id":  pfID,
		"destino_id": pjID,
		"valor":      30.00,
	}, http.StatusOK)

	postJSON(t, client, baseURL+"/conta/"+pfID+"/deposito", "", map[string]any{"valor": 10.00}, http.StatusForbidden)

	deleteJSON(t, client, baseURL+"/conta/"+pfID, token, http.StatusOK)
	deleteJSON(t, client, baseURL+"/conta/"+pjID, token, http.StatusOK)
	getJSON(t, client, baseURL+"/conta/"+pfID+"/saldo", http.StatusNotFound)
}

// getCSRFToken busca o token necessario para rotas que alteram dados.
func getCSRFToken(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()

	body := getJSON(t, client, baseURL+"/csrf-token", http.StatusOK)
	token := stringValue(t, body, "csrf_token")
	if token == "" {
		t.Fatal("expected csrf token")
	}

	return token
}

// getJSON executa chamadas GET e valida o status esperado.
func getJSON(t *testing.T, client *http.Client, url string, wantStatus int) map[string]any {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	return doJSON(t, client, request, wantStatus)
}

func postJSON(t *testing.T, client *http.Client, url string, csrfToken string, payload map[string]any, wantStatus int) map[string]any {
	t.Helper()

	return sendJSON(t, client, http.MethodPost, url, csrfToken, payload, wantStatus)
}

func deleteJSON(t *testing.T, client *http.Client, url string, csrfToken string, wantStatus int) map[string]any {
	t.Helper()

	return sendJSON(t, client, http.MethodDelete, url, csrfToken, nil, wantStatus)
}

// sendJSON monta requisicoes com payload JSON e token CSRF opcional.
func sendJSON(t *testing.T, client *http.Client, method string, url string, csrfToken string, payload map[string]any, wantStatus int) map[string]any {
	t.Helper()

	var body io.Reader
	if payload != nil {
		rawBody, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}
		body = bytes.NewReader(rawBody)
	}

	request, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if csrfToken != "" {
		request.Header.Set("X-CSRF-Token", csrfToken)
	}

	return doJSON(t, client, request, wantStatus)
}

// doJSON executa a requisicao e valida status e corpo JSON.
func doJSON(t *testing.T, client *http.Client, request *http.Request, wantStatus int) map[string]any {
	t.Helper()

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if response.StatusCode != wantStatus {
		t.Fatalf("expected status %d, got %d with body %s", wantStatus, response.StatusCode, body)
	}

	if len(body) == 0 {
		return map[string]any{}
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("expected json response: %v, body: %s", err, body)
	}

	return data
}

func stringValue(t *testing.T, data map[string]any, key string) string {
	t.Helper()

	value, ok := data[key].(string)
	if !ok {
		t.Fatalf("expected string field %q, got %#v", key, data[key])
	}

	return value
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
