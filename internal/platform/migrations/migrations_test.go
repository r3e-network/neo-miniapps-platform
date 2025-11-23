package migrations

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestApplyExecutesAllMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	entries, err := files.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	for range entries {
		mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))
	}

	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestMigrationsAreSorted(t *testing.T) {
	entries, err := files.ReadDir(".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if name := entry.Name(); strings.HasSuffix(name, ".sql") {
			names = append(names, name)
		}
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	if len(sorted) != len(names) {
		t.Fatalf("expected %d migrations, got %d", len(names), len(sorted))
	}
	for i := range names {
		if names[i] != sorted[i] {
			t.Fatalf("migration order mismatch at %d: got %s want %s", i, names[i], sorted[i])
		}
	}
}
