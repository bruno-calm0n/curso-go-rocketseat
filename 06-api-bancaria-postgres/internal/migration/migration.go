package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
)

// Run executa os arquivos .sql em ordem alfabetica.
func Run(db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}

	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		if _, err := db.Exec(string(content)); err != nil {
			return err
		}
	}

	return nil
}
