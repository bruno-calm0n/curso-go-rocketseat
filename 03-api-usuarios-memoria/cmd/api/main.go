package main

import (
	"log"
	"net/http"

	"api-usuarios-memoria/internal/handler"
	"api-usuarios-memoria/internal/store"
)

func main() {
	// Monta a aplicacao usando armazenamento em memoria.
	userStore := store.NewMemoryUserStore()
	userHandler := handler.NewUserHandler(userStore)

	mux := http.NewServeMux()
	userHandler.RegisterRoutes(mux)

	log.Println("server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
