package main

import (
	"database/sql"
	"log"
	"net/http"

	"api-tarefas-paginacao-testes/internal/tasks"

	_ "modernc.org/sqlite"
)

func main() {
	// SQLite local mantem o projeto simples e sem Docker.
	db, err := sql.Open("sqlite", "tasks.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store := tasks.NewStore(db)
	if err := store.Migrate(); err != nil {
		log.Fatal(err)
	}

	handler := tasks.NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	log.Println("server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
