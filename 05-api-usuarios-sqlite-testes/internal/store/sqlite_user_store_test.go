package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"api-usuarios-sqlite-testes/internal/domain"

	_ "modernc.org/sqlite"
)

func TestSQLiteUserStoreCRUD(t *testing.T) {
	// Exercita o ciclo completo do store sem passar pela camada HTTP.
	userStore, _ := newTestStore(t)

	createdUser, err := userStore.Insert(validUser())
	if err != nil {
		t.Fatalf("expected insert to succeed: %v", err)
	}
	if createdUser.ID == "" {
		t.Fatal("expected generated user id")
	}

	users, err := userStore.FindAll()
	if err != nil {
		t.Fatalf("expected find all to succeed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	foundUser, err := userStore.FindByID(createdUser.ID)
	if err != nil {
		t.Fatalf("expected find by id to succeed: %v", err)
	}
	if foundUser == nil || foundUser.FirstName != "Jane" {
		t.Fatalf("expected to find Jane, got %#v", foundUser)
	}

	updatedUser, err := userStore.Update(createdUser.ID, domain.User{
		FirstName: "Maria",
		LastName:  "Silva",
		Biography: "Uma biografia valida para atualizar o usuario.",
	})
	if err != nil {
		t.Fatalf("expected update to succeed: %v", err)
	}
	if updatedUser == nil || updatedUser.LastName != "Silva" {
		t.Fatalf("expected updated user, got %#v", updatedUser)
	}

	deletedUser, err := userStore.Delete(createdUser.ID)
	if err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}
	if deletedUser == nil || deletedUser.ID != createdUser.ID {
		t.Fatalf("expected deleted user, got %#v", deletedUser)
	}

	foundUser, err = userStore.FindByID(createdUser.ID)
	if err != nil {
		t.Fatalf("expected find by id after delete to succeed: %v", err)
	}
	if foundUser != nil {
		t.Fatalf("expected user to be deleted, got %#v", foundUser)
	}
}

func TestSQLiteUserStoreNotFound(t *testing.T) {
	userStore, _ := newTestStore(t)

	foundUser, err := userStore.FindByID("unknown-id")
	if err != nil {
		t.Fatalf("expected find by id to succeed: %v", err)
	}
	if foundUser != nil {
		t.Fatalf("expected nil user, got %#v", foundUser)
	}

	updatedUser, err := userStore.Update("unknown-id", validUser())
	if err != nil {
		t.Fatalf("expected update to succeed: %v", err)
	}
	if updatedUser != nil {
		t.Fatalf("expected nil updated user, got %#v", updatedUser)
	}

	deletedUser, err := userStore.Delete("unknown-id")
	if err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}
	if deletedUser != nil {
		t.Fatalf("expected nil deleted user, got %#v", deletedUser)
	}
}

func TestSQLiteUserStorePersistsData(t *testing.T) {
	// Reabre o mesmo arquivo para garantir persistencia em disco.
	dbPath := filepath.Join(t.TempDir(), "users.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("expected database open to succeed: %v", err)
	}

	userStore := NewSQLiteUserStore(db)
	if err := userStore.Migrate(); err != nil {
		t.Fatalf("expected migrate to succeed: %v", err)
	}

	createdUser, err := userStore.Insert(validUser())
	if err != nil {
		t.Fatalf("expected insert to succeed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("expected first database close to succeed: %v", err)
	}

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("expected database reopen to succeed: %v", err)
	}
	defer db.Close()

	userStore = NewSQLiteUserStore(db)
	if err := userStore.Migrate(); err != nil {
		t.Fatalf("expected second migrate to succeed: %v", err)
	}

	foundUser, err := userStore.FindByID(createdUser.ID)
	if err != nil {
		t.Fatalf("expected find by id to succeed: %v", err)
	}
	if foundUser == nil || foundUser.ID != createdUser.ID {
		t.Fatalf("expected persisted user, got %#v", foundUser)
	}
}

func TestSQLiteUserStoreUsesSafeQueryArguments(t *testing.T) {
	// Confirma que entradas maliciosas sao tratadas como argumentos da query.
	userStore, _ := newTestStore(t)

	_, err := userStore.Insert(validUser())
	if err != nil {
		t.Fatalf("expected insert to succeed: %v", err)
	}

	foundUser, err := userStore.FindByID("' OR 1=1 --")
	if err != nil {
		t.Fatalf("expected malicious id lookup to succeed safely: %v", err)
	}
	if foundUser != nil {
		t.Fatalf("expected malicious id to find nothing, got %#v", foundUser)
	}

	users, err := userStore.FindAll()
	if err != nil {
		t.Fatalf("expected find all to succeed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected table to remain intact with 1 user, got %d", len(users))
	}
}

func TestSQLiteUserStoreMigrateCanRunMoreThanOnce(t *testing.T) {
	userStore, _ := newTestStore(t)

	if err := userStore.Migrate(); err != nil {
		t.Fatalf("expected second migrate to succeed: %v", err)
	}
}

func newTestStore(t *testing.T) (*SQLiteUserStore, *sql.DB) {
	t.Helper()

	// Banco temporario deixa cada teste independente.
	dbPath := filepath.Join(t.TempDir(), "users.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("expected database open to succeed: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	userStore := NewSQLiteUserStore(db)
	if err := userStore.Migrate(); err != nil {
		t.Fatalf("expected migrate to succeed: %v", err)
	}

	return userStore, db
}

func validUser() domain.User {
	return domain.User{
		FirstName: "Jane",
		LastName:  "Doe",
		Biography: "Tendo diversao estudando Go todos os dias.",
	}
}
