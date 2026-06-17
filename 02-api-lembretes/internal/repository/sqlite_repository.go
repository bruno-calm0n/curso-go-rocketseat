package repository

import (
	"database/sql"
	"errors"
	"time"

	"api-lembretes/internal/domain"
)

var ErrNoteNotFound = errors.New("note not found")

type SqliteRepository struct {
	db *sql.DB
}

// NewSqliteRepository recebe a conexao pronta por injecao de dependencia.
func NewSqliteRepository(db *sql.DB) *SqliteRepository {
	return &SqliteRepository{db: db}
}

// Migrate garante que a tabela de lembretes exista ao iniciar a API.
func (r *SqliteRepository) Migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
	`

	_, err := r.db.Exec(query)
	return err
}

// Create salva um novo lembrete e devolve o registro com id preenchido.
func (r *SqliteRepository) Create(note domain.Note) (domain.Note, error) {
	note.CreatedAt = time.Now().UTC()

	result, err := r.db.Exec(
		"INSERT INTO notes (title, content, created_at) VALUES (?, ?, ?)",
		note.Title,
		note.Content,
		note.CreatedAt,
	)
	if err != nil {
		return domain.Note{}, err
	}

	note.ID, err = result.LastInsertId()
	if err != nil {
		return domain.Note{}, err
	}

	return note, nil
}

// FindAll lista os lembretes mais recentes primeiro.
func (r *SqliteRepository) FindAll() ([]domain.Note, error) {
	rows, err := r.db.Query("SELECT id, title, content, created_at FROM notes ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]domain.Note, 0)
	for rows.Next() {
		var note domain.Note
		if err := rows.Scan(&note.ID, &note.Title, &note.Content, &note.CreatedAt); err != nil {
			return nil, err
		}

		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

// FindByID busca um lembrete e converte sql.ErrNoRows em erro de dominio.
func (r *SqliteRepository) FindByID(id int64) (domain.Note, error) {
	var note domain.Note

	err := r.db.QueryRow(
		"SELECT id, title, content, created_at FROM notes WHERE id = ?",
		id,
	).Scan(&note.ID, &note.Title, &note.Content, &note.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Note{}, ErrNoteNotFound
	}
	if err != nil {
		return domain.Note{}, err
	}

	return note, nil
}

// Update altera titulo e conteudo mantendo a data original de criacao.
func (r *SqliteRepository) Update(note domain.Note) (domain.Note, error) {
	currentNote, err := r.FindByID(note.ID)
	if err != nil {
		return domain.Note{}, err
	}

	// Mantem a data original de criacao ao atualizar titulo e conteudo.
	note.CreatedAt = currentNote.CreatedAt

	result, err := r.db.Exec(
		"UPDATE notes SET title = ?, content = ? WHERE id = ?",
		note.Title,
		note.Content,
		note.ID,
	)
	if err != nil {
		return domain.Note{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return domain.Note{}, err
	}
	if rowsAffected == 0 {
		return domain.Note{}, ErrNoteNotFound
	}

	return note, nil
}

// Delete remove o lembrete e informa quando o id nao existe.
func (r *SqliteRepository) Delete(id int64) error {
	result, err := r.db.Exec("DELETE FROM notes WHERE id = ?", id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNoteNotFound
	}

	return nil
}
