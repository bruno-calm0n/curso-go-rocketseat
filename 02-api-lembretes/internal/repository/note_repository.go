package repository

import "api-lembretes/internal/domain"

// NoteRepository define o contrato usado pelo controller.
type NoteRepository interface {
	Create(note domain.Note) (domain.Note, error)
	FindAll() ([]domain.Note, error)
	FindByID(id int64) (domain.Note, error)
	Update(note domain.Note) (domain.Note, error)
	Delete(id int64) error
}
