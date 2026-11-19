package egress

import (
	"strings"
	"testing"
)

func TestExpandProxySID(t *testing.T) {
	raw := "host:1:user-{sid}:pass"
	got := ExpandProxySID(raw)
	if strings.Contains(got, "{sid}") {
		t.Fatalf("sid not expanded: %s", got)
	}
	if !strings.HasPrefix(got, "host:1:user-") {
		t.Fatalf("unexpected: %s", got)
	}
	if ExpandProxySID("no-sid") != "no-sid" {
		t.Fatal("unchanged expected")
	}
}

func TestPickListNodeDefaultSocks(t *testing.T) {
	n, err := PickListNode("1.2.3.4:1080:u:p")
	if err != nil {
		t.Fatal(err)
	}
	if n.ProxyType != "socks5" {
		t.Fatalf("want socks5 got %s", n.ProxyType)
	}
	if n.Host != "1.2.3.4" || n.Port != 1080 || n.Username != "u" {
		t.Fatalf("parsed %#v", n)
	}
	if n.ID != 0 {
		t.Fatalf("ephemeral id should be 0")
	}
}

func TestNormalizeModeList(t *testing.T) {
	if NormalizeMode("list") != ModeList {
		t.Fatal("list")
	}
	if NormalizeMode("muse") != ModeList {
		t.Fatal("muse")
	}
	if NormalizeMode("direct") != ModeDirect {
		t.Fatal("direct")
	}
	if NormalizeMode("") != ModePool {
		t.Fatal("default pool")
	}
}
