package egress

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

const defaultTimeout = 120 * time.Second

// Session is a RH client bound to an optional egress node.
type Session struct {
	Client *runninghub.Client
	Node   *store.EgressNode // nil = direct
}

// Manager resolves sticky egress and builds clients.
type Manager struct {
	Store *store.Store
}

func NewManager(st *store.Store) *Manager {
	return &Manager{Store: st}
}

// Resolve returns bound egress or picks a new one and binds it.
// excludeID is used when rebinding after failure.
func (m *Manager) Resolve(acc store.Account, excludeID int64) (*store.EgressNode, error) {
	if acc.EgressID > 0 && acc.EgressID != excludeID {
		n, err := m.Store.GetEgress(acc.EgressID)
		if err == nil && n.Status == "可用" {
			return &n, nil
		}
		// bound node gone / failed — clear
		_ = m.Store.SetAccountEgress(acc.ID, 0)
	}
	n, err := m.Store.PickAvailableEgress(excludeID)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, nil // direct
		}
		return nil, err
	}
	if err := m.Store.SetAccountEgress(acc.ID, n.ID); err != nil {
		return nil, err
	}
	_ = m.Store.MarkEgressUsed(n.ID)
	return &n, nil
}

// BuildHTTPClient builds an http.Client that dials via the egress node (or direct).
func BuildHTTPClient(n *store.EgressNode, timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if n == nil || strings.TrimSpace(n.Host) == "" {
		return &http.Client{Timeout: timeout}, nil
	}
	transport := &http.Transport{
		Proxy:                 nil,
		MaxIdleConns:          32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	ptype := strings.ToLower(strings.TrimSpace(n.ProxyType))
	switch ptype {
	case "socks5", "socks", "socks5h":
		var auth *proxy.Auth
		if n.Username != "" || n.Password != "" {
			auth = &proxy.Auth{User: n.Username, Password: n.Password}
		}
		addr := net.JoinHostPort(n.Host, fmt.Sprintf("%d", n.Port))
		dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5 dialer: %w", err)
		}
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = cd.DialContext
		} else {
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.Dial(network, address)
			}
		}
	default:
		u := &url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(n.Host, fmt.Sprintf("%d", n.Port)),
		}
		if ptype == "https" {
			u.Scheme = "https"
		}
		if n.Username != "" || n.Password != "" {
			u.User = url.UserPassword(n.Username, n.Password)
		}
		transport.Proxy = http.ProxyURL(u)
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

// NewClientForAccount builds a RH client with sticky egress + fingerprint.
// On connectivity failure callers should use IsConnectivityError + Rebind.
func (m *Manager) NewClientForAccount(acc store.Account, baseURL string) (*Session, error) {
	node, err := m.Resolve(acc, 0)
	if err != nil {
		return nil, err
	}
	httpClient, err := BuildHTTPClient(node, defaultTimeout)
	if err != nil {
		return nil, err
	}
	client := runninghub.NewClient(baseURL)
	client.HTTP = httpClient
	client.ApplyIdentity(acc.AnonymousID, acc.FingerprintJSON)
	if node == nil {
		log.Printf("[egress] account=%d email=%s direct (no available egress)", acc.ID, acc.Email)
	} else {
		log.Printf("[egress] account=%d email=%s bind=%s#%d %s", acc.ID, acc.Email, node.Label(), node.ID, node.ProxyType)
	}
	return &Session{Client: client, Node: node}, nil
}

// Rebind clears sticky binding after connectivity failure and returns a new session.
func (m *Manager) Rebind(acc store.Account, baseURL string, failed *store.EgressNode, reason string) (*Session, error) {
	exclude := int64(0)
	if failed != nil {
		exclude = failed.ID
		_, _ = m.Store.MarkEgressFailure(failed.ID, reason)
	}
	_ = m.Store.SetAccountEgress(acc.ID, 0)
	acc.EgressID = 0
	node, err := m.Resolve(acc, exclude)
	if err != nil {
		return nil, err
	}
	httpClient, err := BuildHTTPClient(node, defaultTimeout)
	if err != nil {
		return nil, err
	}
	client := runninghub.NewClient(baseURL)
	client.HTTP = httpClient
	client.ApplyIdentity(acc.AnonymousID, acc.FingerprintJSON)
	return &Session{Client: client, Node: node}, nil
}

// MarkOK records egress success after a successful RH call.
func (m *Manager) MarkOK(sess *Session) {
	if sess == nil || sess.Node == nil {
		return
	}
	_ = m.Store.MarkEgressSuccess(sess.Node.ID, 0)
}

// Do runs fn with a sticky-egress client; on connectivity failure rebinds once and retries.
func (m *Manager) Do(acc store.Account, baseURL string, fn func(*runninghub.Client) error) error {
	sess, err := m.NewClientForAccount(acc, baseURL)
	if err != nil {
		return err
	}
	err = fn(sess.Client)
	if err == nil {
		m.MarkOK(sess)
		return nil
	}
	if !IsConnectivityError(err) || sess.Node == nil {
		return err
	}
	log.Printf("[egress] connectivity fail account=%d node=%d: %v — rebind", acc.ID, sess.Node.ID, err)
	sess2, err2 := m.Rebind(acc, baseURL, sess.Node, err.Error())
	if err2 != nil {
		return err
	}
	err = fn(sess2.Client)
	if err == nil {
		m.MarkOK(sess2)
	}
	return err
}

// AttachHTTP sets RunInput.HTTP from a sticky session (caller still owns login).
func AttachHTTP(in *runninghub.RunInput, sess *Session) {
	if in == nil || sess == nil || sess.Client == nil {
		return
	}
	in.HTTP = sess.Client.HTTP
	if sess.Client.AnonymousID != "" {
		in.AnonymousID = sess.Client.AnonymousID
	}
}

// IsConnectivityError reports dial/proxy/transport failures that warrant rebind.
func IsConnectivityError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	needles := []string{
		"proxy",
		"dial tcp",
		"dial udp",
		"i/o timeout",
		"connection refused",
		"connection reset",
		"no such host",
		"network is unreachable",
		"tls handshake timeout",
		"eof",
		"broken pipe",
		"socks connect",
		"socks5",
		"context deadline exceeded",
		"timeout awaiting response",
		"connectex",
		"wsarecv",
		"wsasend",
	}
	for _, n := range needles {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

// Probe tests connectivity through the node to targetURL; returns latency ms.
func Probe(n store.EgressNode, targetURL string) (latencyMs int, err error) {
	if targetURL == "" {
		targetURL = "https://www.runninghub.ai"
	}
	client, err := BuildHTTPClient(&n, 25*time.Second)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "runninghub2api-egress-probe/1.0")
	resp, err := client.Do(req)
	latencyMs = int(time.Since(start).Milliseconds())
	if err != nil {
		return latencyMs, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return latencyMs, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	return latencyMs, nil
}
