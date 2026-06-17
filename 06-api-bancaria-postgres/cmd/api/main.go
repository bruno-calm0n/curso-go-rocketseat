package main

import (
	"database/sql"
	"log"
	"net/http"

	"api-bancaria-postgres/internal/config"
	"api-bancaria-postgres/internal/httpapi"
	"api-bancaria-postgres/internal/migration"
	"api-bancaria-postgres/internal/repository"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// Carrega configuracoes por variaveis de ambiente com fallback local.
	cfg := config.Load()

	// Abre a conexao com PostgreSQL local usando o driver pgx.
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	// Executa os arquivos SQL para garantir o schema do banco.
	if err := migration.Run(db, cfg.MigrationsDir); err != nil {
		log.Fatal(err)
	}

	// Liga repositorio, handlers e protecao CSRF por injecao de dependencia.
	accountRepository := repository.NewPostgresAccountRepository(db)
	csrfManager := httpapi.NewCSRFManager(cfg.CSRFSecret)
	accountHandler := httpapi.NewAccountHandler(accountRepository)

	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, accountHandler, csrfManager)

	log.Println("server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
