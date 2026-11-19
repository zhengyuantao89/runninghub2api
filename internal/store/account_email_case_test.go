package store

import (
	"path/filepath"
	"testing"
)

func TestUpsertAccountPreservesEmailCase(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	mixed := "EldenCaraza135167@outlook.com"
	acc, err := s.UpsertAccount(Account{
		Email:    mixed,
		Password: "secret",
		Status:   "active",
		Source:   "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if acc.Email != mixed {
		t.Fatalf("email casing lost: got %q want %q", acc.Email, mixed)
	}

	// Re-upsert with same email different case should match (NOCASE) and restore/update casing.
	again := "eldencaraza135167@outlook.com"
	acc2, err := s.UpsertAccount(Account{
		Email:    again,
		Password: "secret2",
		Status:   "active",
		Source:   "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if acc2.ID != acc.ID {
		t.Fatalf("expected same row id=%d got=%d", acc.ID, acc2.ID)
	}
	if acc2.Email != again {
		t.Fatalf("email not updated to new casing: got %q want %q", acc2.Email, again)
	}

	byMixed, err := s.GetAccountByEmail(mixed)
	if err != nil {
		t.Fatal(err)
	}
	if byMixed.ID != acc.ID {
		t.Fatalf("GetAccountByEmail NOCASE failed")
	}
}
