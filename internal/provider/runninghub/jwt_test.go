package runninghub

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestDecodeJWTExpAndFresh(t *testing.T) {
	exp := time.Now().UTC().Add(48 * time.Hour).Unix()
	payload, _ := json.Marshal(map[string]any{"sub": "u1", "exp": exp})
	token := "hdr." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	got := DecodeJWTExp(token)
	if got.IsZero() || got.Unix() != exp {
		t.Fatalf("DecodeJWTExp=%v want unix %d", got, exp)
	}
	if !AccessTokenFresh(token, 24*time.Hour) {
		t.Fatal("expected fresh with 24h skew")
	}
	if AccessTokenFresh(token, 72*time.Hour) {
		t.Fatal("expected stale with 72h skew")
	}
	if AccessTokenFresh("", time.Hour) {
		t.Fatal("empty token must not be fresh")
	}
	if !AccessTokenFresh("not-a-jwt", time.Hour) {
		t.Fatal("opaque token without exp should be treated as usable")
	}
}
