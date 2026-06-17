package main

import (
	"database/sql"
	"log"
	"net/http"

	"api-usuarios-sqlite-testes/internal/handler"
	"api-usuarios-sqlite-testes/internal/store"

	_ "modernc.org/sqlite"
)

func main() {
	// Abre o arquivo SQLite local usado para persistir usuarios.
	db, err := sql.Open("sqlite", "users.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Injeta a conexao no store e prepara a tabela.
	userStore := store.NewSQLiteUserStore(db)
	if err := userStore.Migrate(); err != nil {
		log.Fatal(err)
	}

	// O handler conhece apenas a interface do store.
	userHandler := handler.NewUserHandler(userStore)

	mux := http.NewServeMux()
	userHandler.RegisterRoutes(mux)

	log.Println("server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
