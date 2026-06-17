package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"api-auth-jwt-refresh/internal/auth"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := getenv("DB_PATH", "auth.db")
	jwtSecret := getenv("JWT_SECRET", "dev-secret-change-me")

	// Abre o SQLite local usado para usuarios e refresh tokens.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store := auth.NewStore(db)
	if err := store.Migrate(); err != nil {
		log.Fatal(err)
	}

	// O handler recebe dependencias prontas, sem globais.
	tokenManager := auth.NewTokenManager(jwtSecret)
	handler := auth.NewHandler(store, tokenManager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	log.Println("server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
