package store

import (
	"api-usuarios-memoria/internal/domain"
)

// UserStore define o contrato usado pelos handlers HTTP.
type UserStore interface {
	FindAll() ([]domain.User, error)
	FindByID(id string) (*domain.User, error)
	Insert(newUser domain.User) (domain.User, error)
	Update(id string, userUpdates domain.User) (*domain.User, error)
	Delete(id string) (*domain.User, error)
}
