package httpapi

import "net/http"

// RegisterRoutes registra rotas publicas e protege escritas com CSRF.
func RegisterRoutes(mux *http.ServeMux, accountHandler *AccountHandler, csrfManager *CSRFManager) {
	mux.HandleFunc("GET /csrf-token", func(w http.ResponseWriter, r *http.Request) {
		token, err := csrfManager.Generate()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate csrf token")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"csrf_token": token})
	})

	mux.Handle("POST /conta", csrfManager.Middleware(http.HandlerFunc(accountHandler.CreateAccount)))
	mux.HandleFunc("GET /conta/{id}/saldo", accountHandler.GetBalance)
	mux.Handle("POST /conta/{id}/deposito", csrfManager.Middleware(http.HandlerFunc(accountHandler.Deposit)))
	mux.Handle("POST /conta/{id}/saque", csrfManager.Middleware(http.HandlerFunc(accountHandler.Withdraw)))
	mux.Handle("POST /conta/transferencia", csrfManager.Middleware(http.HandlerFunc(accountHandler.Transfer)))
	mux.Handle("DELETE /conta/{id}", csrfManager.Middleware(http.HandlerFunc(accountHandler.CloseAccount)))
}
