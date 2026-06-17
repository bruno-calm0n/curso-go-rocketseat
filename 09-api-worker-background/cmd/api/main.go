package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"api-worker-background/internal/jobs"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "jobs.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store := jobs.NewStore(db)
	if err := store.Migrate(); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Worker roda junto com a API e processa jobs pendentes.
	worker := jobs.NewWorker(store, 1*time.Second)
	go worker.Start(ctx)

	handler := jobs.NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		log.Println("server running on http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)
}
