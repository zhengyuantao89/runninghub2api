package egress

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/trae/runninghub2api/internal/store"
)

const SettingProxies = "egress.proxies"

// ExpandProxySID replaces {sid} with an 8-char sticky session token (muse-style).
func ExpandProxySID(raw string) string {
	if raw == "" || !strings.Contains(raw, "{sid}") {
		return raw
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			b[i] = alphabet[i%len(alphabet)]
			continue
		}
		b[i] = alphabet[n.Int64()]
	}
	return strings.ReplaceAll(raw, "{sid}", string(b))
}

// SplitProxyLines splits muse-style proxies setting into non-empty lines.
func SplitProxyLines(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, ",", "\n")
	out := make([]string, 0, 8)
	for _, part := range strings.Split(raw, "\n") {
		p := strings.TrimSpace(part)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ParseListLine parses one muse-style proxy line into an ephemeral egress node.
// No scheme → socks5 (muse default socks5h). {sid} should already be expanded.
func ParseListLine(raw string) (store.EgressNode, bool) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return store.EgressNode{}, false
	}
	n, ok := parseProxyLine(line, "list")
	if !ok {
		return store.EgressNode{}, false
	}
	if !strings.Contains(line, "://") {
		n.ProxyType = "socks5"
	}
	n.ID = 0
	n.Source = "list"
	n.Remark = "list"
	n.Status = "可用"
	return n, true
}

// PickListNode randomly picks one line from proxies text, expands {sid}, returns ephemeral node.
func PickListNode(raw string) (*store.EgressNode, error) {
	lines := SplitProxyLines(raw)
	if len(lines) == 0 {
		return nil, store.ErrNotFound
	}
	idx := 0
	if len(lines) > 1 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(lines))))
		if err == nil {
			idx = int(n.Int64())
		}
	}
	expanded := ExpandProxySID(lines[idx])
	node, ok := ParseListLine(expanded)
	if !ok {
		return nil, fmt.Errorf("invalid proxy line: %s", maskProxyLine(lines[idx]))
	}
	return &node, nil
}

func maskProxyLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return "(empty)"
	}
	if i := strings.Index(line, "://"); i >= 0 {
		rest := line[i+3:]
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			return line[:i+3] + "***:***@" + rest[at+1:]
		}
	}
	parts := strings.Split(line, ":")
	if len(parts) >= 4 {
		return parts[0] + ":" + parts[1] + ":***:***"
	}
	return line
}

func (m *Manager) proxiesRaw() string {
	if m == nil || m.Store == nil {
		return ""
	}
	v, _ := m.Store.GetSetting(SettingProxies)
	return v
}

// PickListForUse loads settings.proxies and picks one ephemeral node.
func (m *Manager) PickListForUse() (*store.EgressNode, error) {
	return PickListNode(m.proxiesRaw())
}
