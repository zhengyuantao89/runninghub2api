package store

import (
	"path/filepath"
	"testing"
)

func TestListAccountsDispatchableFilter(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ok, err := s.UpsertAccount(Account{
		Email: "ok@icloud.com", Password: "p", Status: "active", Source: "manual",
		RemainCoins: "10", LastBalanceAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	paused, err := s.UpsertAccount(Account{
		Email: "paused@icloud.com", Password: "p", Status: "active", Source: "manual",
		RemainCoins: "10", LastBalanceAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PauseAccountDispatch(paused.ID); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListAccountsPage(0, 0, "", "", "dispatchable", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != ok.ID {
		t.Fatalf("dispatchable want 1 ok account, got %+v", list)
	}

	pausedList, err := s.ListAccountsPage(0, 0, "", "", "paused", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pausedList) != 1 || pausedList[0].ID != paused.ID {
		t.Fatalf("paused want 1, got %+v", pausedList)
	}
}
