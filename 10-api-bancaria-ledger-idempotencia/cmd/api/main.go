package main

import (
	"database/sql"
	"log"
	"net/http"

	"api-bancaria-ledger-idempotencia/internal/bank"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "bank.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store := bank.NewStore(db)
	if err := store.Migrate(); err != nil {
		log.Fatal(err)
	}

	handler := bank.NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	log.Println("server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
