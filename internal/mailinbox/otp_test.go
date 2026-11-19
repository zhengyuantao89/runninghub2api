package mailinbox

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseInboxCodeJSON(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
		ok   bool
	}{
		{"no_code", `{"success":false,"code":"no_code","message":"暂未收到验证码","retryable":true}`, "", true},
		{"ok_digits", `{"success":true,"code":"482915","message":"ok"}`, "482915", true},
		{"success_false_digits", `{"success":false,"code":"482915"}`, "", true},
		{"code_only", `{"code":"482915"}`, "", false},
		{"not_shape", `{"foo":1}`, "", false},
		{"plain_text", `hello 123456`, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, ok := parseInboxCodeJSON([]byte(tc.body))
			if ok != tc.ok || code != tc.code {
				t.Fatalf("got code=%q ok=%v want code=%q ok=%v", code, ok, tc.code, tc.ok)
			}
		})
	}
}

func TestWaitOTPIgnoresBaseline(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Header().Set("Content-Type", "application/json")
		if n < 3 {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "code": "111111"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "code": "222222"})
	}))
	defer srv.Close()

	got, err := WaitOTP(WaitOpts{
		PickupURL: srv.URL,
		Timeout:   5 * time.Second,
		Interval:  50 * time.Millisecond,
		Baseline:  "111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "222222" {
		t.Fatalf("got %q want 222222", got)
	}
}
