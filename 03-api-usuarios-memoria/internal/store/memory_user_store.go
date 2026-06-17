package store

import (
	"crypto/rand"
	"fmt"
	"sync"

	"api-usuarios-memoria/internal/domain"
)

type MemoryUserStore struct {
	mu   sync.RWMutex
	data map[string]domain.User
}

// NewMemoryUserStore cria o mapa usado como banco em memoria.
func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		data: make(map[string]domain.User),
	}
}

// FindAll copia os usuarios do mapa sem expor a estrutura interna.
func (s *MemoryUserStore) FindAll() ([]domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]domain.User, 0, len(s.data))
	for _, user := range s.data {
		users = append(users, user)
	}

	return users, nil
}

// FindByID retorna nil quando o usuario nao existe.
func (s *MemoryUserStore) FindByID(id string) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.data[id]
	if !exists {
		return nil, nil
	}

	return &user, nil
}

// Insert gera um UUID e salva o usuario no mapa.
func (s *MemoryUserStore) Insert(newUser domain.User) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := newUUID()
	if err != nil {
		return domain.User{}, err
	}

	newUser.ID = id
	s.data[newUser.ID] = newUser

	return newUser, nil
}

// Update altera apenas os campos editaveis do usuario.
func (s *MemoryUserStore) Update(id string, userUpdates domain.User) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentUser, exists := s.data[id]
	if !exists {
		return nil, nil
	}

	currentUser.FirstName = userUpdates.FirstName
	currentUser.LastName = userUpdates.LastName
	currentUser.Biography = userUpdates.Biography
	s.data[id] = currentUser

	return &currentUser, nil
}

// Delete remove o usuario e devolve o registro deletado.
func (s *MemoryUserStore) Delete(id string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.data[id]
	if !exists {
		return nil, nil
	}

	delete(s.data, id)
	return &user, nil
}

// newUUID cria um identificador unico sem depender de biblioteca externa.
func newUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Ajusta os bits para gerar um UUID versao 4 valido.
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		bytes[0:4],
		bytes[4:6],
		bytes[6:8],
		bytes[8:10],
		bytes[10:16],
	), nil
}
