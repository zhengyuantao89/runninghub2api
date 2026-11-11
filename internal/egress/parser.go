package egress

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/trae/runninghub2api/internal/store"
)

// ParseProxies parses plain-text proxy lines into egress nodes.
// Supported:
//
//	type://user:pass@host:port
//	user:pass@host:port
//	host:port:user:pass
//	host:port
func ParseProxies(text, group string) []store.EgressNode {
	group = strings.TrimSpace(group)
	if group == "" {
		group = NodesGroup
	}
	lines := strings.Split(text, "\n")
	out := make([]store.EgressNode, 0, len(lines))
	seen := map[string]struct{}{}
	for _, raw := range lines {
		n, ok := parseProxyLine(raw, group)
		if !ok {
			continue
		}
		fp := Fingerprint(n)
		if _, dup := seen[fp]; dup {
			continue
		}
		seen[fp] = struct{}{}
		out = append(out, n)
	}
	return out
}

func parseProxyLine(raw, group string) (store.EgressNode, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return store.EgressNode{}, false
	}
	var (
		proxyType = "http"
		host      string
		port      int
		username  string
		password  string
	)
	if strings.Contains(line, "://") {
		u, err := url.Parse(line)
		if err != nil || u.Hostname() == "" {
			return store.EgressNode{}, false
		}
		proxyType = normalizeProxyType(strings.TrimSuffix(u.Scheme, ":"))
		if proxyType == "" {
			return store.EgressNode{}, false
		}
		host = u.Hostname()
		port, _ = strconv.Atoi(u.Port())
		if u.User != nil {
			username = u.User.Username()
			password, _ = u.User.Password()
		}
	} else if at := strings.LastIndex(line, "@"); at >= 0 {
		auth := line[:at]
		addr := line[at+1:]
		h, p, ok := splitHostPort(addr)
		if !ok {
			return store.EgressNode{}, false
		}
		host, port = h, p
		if cut := strings.IndexByte(auth, ':'); cut >= 0 {
			username = auth[:cut]
			password = auth[cut+1:]
		} else {
			username = auth
		}
	} else {
		parts := strings.Split(line, ":")
		if len(parts) >= 4 {
			host = parts[0]
			port, _ = strconv.Atoi(parts[1])
			username = parts[2]
			password = strings.Join(parts[3:], ":")
		} else if len(parts) >= 2 {
			host = parts[0]
			port, _ = strconv.Atoi(parts[1])
		} else {
			return store.EgressNode{}, false
		}
	}
	host = strings.TrimSpace(host)
	if host == "" || port < 1 || port > 65535 {
		return store.EgressNode{}, false
	}
	return store.EgressNode{
		GroupName: group,
		ProxyType: proxyType,
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
		Status:    "可用",
		Source:    "subscription",
		Remark:    "subscription:" + group,
	}, true
}

func splitHostPort(addr string) (string, int, bool) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		// host:port without brackets
		colon := strings.LastIndex(addr, ":")
		if colon < 0 {
			return "", 0, false
		}
		host = addr[:colon]
		portStr = addr[colon+1:]
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, false
	}
	return host, port, true
}

func normalizeProxyType(t string) string {
	t = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(t, ":")))
	switch t {
	case "socks5", "socks", "socks5h":
		return "socks5"
	case "https":
		return "https"
	case "http", "":
		return "http"
	default:
		return ""
	}
}

// Fingerprint is the exact-sync identity key for a proxy line.
func Fingerprint(n store.EgressNode) string {
	t := normalizeProxyType(n.ProxyType)
	if t == "" {
		t = "http"
	}
	return strings.ToLower(t) + "|" + strings.ToLower(strings.TrimSpace(n.Host)) + "|" +
		strconv.Itoa(n.Port) + "|" + n.Username
}
