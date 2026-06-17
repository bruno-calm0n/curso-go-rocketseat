package main

import (
	"database/sql"
	"log"
	"net/http"

	"api-lembretes/internal/controller"
	"api-lembretes/internal/repository"

	_ "modernc.org/sqlite"
)

func main() {
	// Abre a conexao SQLite usada pelo repositorio.
	db, err := sql.Open("sqlite", "notes.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Injeta o banco no repositorio e prepara a tabela.
	noteRepository := repository.NewSqliteRepository(db)
	if err := noteRepository.Migrate(); err != nil {
		log.Fatal(err)
	}

	// O controller depende apenas da interface do repositorio.
	noteController := controller.NewNoteController(noteRepository)

	mux := http.NewServeMux()
	noteController.RegisterRoutes(mux)

	log.Println("server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
