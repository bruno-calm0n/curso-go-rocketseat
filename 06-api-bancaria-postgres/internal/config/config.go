package config

import "os"

type Config struct {
	DatabaseURL   string
	CSRFSecret    string
	MigrationsDir string
}

// Load centraliza as configuracoes da aplicacao.
func Load() Config {
	return Config{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/banco_go?sslmode=disable"),
		CSRFSecret:    getEnv("CSRF_SECRET", "dev-secret-change-me"),
		MigrationsDir: getEnv("MIGRATIONS_DIR", "migrations"),
	}
}

// getEnv usa o fallback quando a variavel nao foi definida.
func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
