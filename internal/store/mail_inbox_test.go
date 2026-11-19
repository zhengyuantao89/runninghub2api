package store

import "testing"

func TestIsICloudEmail(t *testing.T) {
	if !IsICloudEmail("Foo@iCloud.com") {
		t.Fatal("expected icloud ok")
	}
	if IsICloudEmail("a@me.com") || IsICloudEmail("a@mac.com") || IsICloudEmail("a@gmail.com") {
		t.Fatal("only @icloud.com allowed")
	}
}

func TestParseInboxImportLine(t *testing.T) {
	email, url, err := ParseInboxImportLine("x@icloud.com----https://example.com/p/1")
	if err != nil || email != "x@icloud.com" || url != "https://example.com/p/1" {
		t.Fatalf("got %q %q %v", email, url, err)
	}
	if _, _, err := ParseInboxImportLine("x@gmail.com----https://example.com/p/1"); err == nil {
		t.Fatal("expected reject non-icloud")
	}
}
