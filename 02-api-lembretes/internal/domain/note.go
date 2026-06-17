package domain

import "time"

// Note representa o lembrete usado em todas as camadas da aplicacao.
type Note struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
