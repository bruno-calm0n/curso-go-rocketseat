package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"

	"api-usuarios-sqlite-testes/internal/domain"
)

type SQLiteUserStore struct {
	db *sql.DB
}

// NewSQLiteUserStore recebe a conexao pronta por injecao de dependencia.
func NewSQLiteUserStore(db *sql.DB) *SQLiteUserStore {
	return &SQLiteUserStore{db: db}
}

// Migrate garante que a tabela users exista antes da API receber chamadas.
func (s *SQLiteUserStore) Migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			biography TEXT NOT NULL
		);
	`

	_, err := s.db.Exec(query)
	return err
}

// FindAll lista todos os usuarios salvos no SQLite.
func (s *SQLiteUserStore) FindAll() ([]domain.User, error) {
	rows, err := s.db.Query(`
		SELECT id, first_name, last_name, biography
		FROM users
		ORDER BY rowid DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Biography); err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// FindByID busca por chave primaria e retorna nil quando nao encontrar.
func (s *SQLiteUserStore) FindByID(id string) (*domain.User, error) {
	var user domain.User

	err := s.db.QueryRow(`
		SELECT id, first_name, last_name, biography
		FROM users
		WHERE id = ?
	`, id).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Biography)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Insert gera um UUID e persiste o novo usuario.
func (s *SQLiteUserStore) Insert(newUser domain.User) (domain.User, error) {
	id, err := newUUID()
	if err != nil {
		return domain.User{}, err
	}

	newUser.ID = id

	// Placeholders evitam SQL injection ao enviar dados para a query.
	_, err = s.db.Exec(`
		INSERT INTO users (id, first_name, last_name, biography)
		VALUES (?, ?, ?, ?)
	`, newUser.ID, newUser.FirstName, newUser.LastName, newUser.Biography)
	if err != nil {
		return domain.User{}, err
	}

	return newUser, nil
}

// Update confirma se o usuario existe antes de alterar os dados.
func (s *SQLiteUserStore) Update(id string, userUpdates domain.User) (*domain.User, error) {
	currentUser, err := s.FindByID(id)
	if err != nil {
		return nil, err
	}
	if currentUser == nil {
		return nil, nil
	}

	_, err = s.db.Exec(`
		UPDATE users
		SET first_name = ?, last_name = ?, biography = ?
		WHERE id = ?
	`, userUpdates.FirstName, userUpdates.LastName, userUpdates.Biography, id)
	if err != nil {
		return nil, err
	}

	updatedUser := domain.User{
		ID:        id,
		FirstName: userUpdates.FirstName,
		LastName:  userUpdates.LastName,
		Biography: userUpdates.Biography,
	}

	return &updatedUser, nil
}

// Delete busca o registro antes de remover para conseguir retorna-lo.
func (s *SQLiteUserStore) Delete(id string) (*domain.User, error) {
	user, err := s.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	_, err = s.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return nil, err
	}

	return user, nil
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
