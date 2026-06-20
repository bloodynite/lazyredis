package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/frankz/lazyredis/internal/config"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/net/proxy"
)

type connResources struct {
	sshClient *ssh.Client
}

func (r *connResources) Close() error {
	if r == nil || r.sshClient == nil {
		return nil
	}
	return r.sshClient.Close()
}

type dialBundle struct {
	dialer    func(ctx context.Context, network, addr string) (net.Conn, error)
	tlsConfig *tls.Config
	resources *connResources
}

type deadlineFreeConn struct {
	net.Conn
}

func (deadlineFreeConn) SetDeadline(time.Time) error      { return nil }
func (deadlineFreeConn) SetReadDeadline(time.Time) error  { return nil }
func (deadlineFreeConn) SetWriteDeadline(time.Time) error { return nil }

func prepareDial(p config.Profile) (*dialBundle, error) {
	tlsCfg, err := buildTLSConfig(p)
	if err != nil {
		return nil, err
	}
	if !needsCustomDial(p) {
		return &dialBundle{
			tlsConfig: tlsCfg,
			resources: &connResources{},
		}, nil
	}

	resources := &connResources{}
	var base proxy.Dialer = proxy.Direct

	if p.Proxy != nil && p.Proxy.Type != "" && p.Proxy.Addr != "" {
		d, err := buildProxyDialer(p.Proxy)
		if err != nil {
			return nil, err
		}
		base = d
	}

	var sshDial func(string, string) (net.Conn, error)
	if p.SSHTunnel != nil && p.SSHTunnel.Enabled {
		client, err := connectSSH(p.SSHTunnel, base)
		if err != nil {
			return nil, err
		}
		resources.sshClient = client
		sshDial = client.Dial
	}

	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		if sshDial != nil {
			type res struct {
				conn net.Conn
				err  error
			}
			ch := make(chan res, 1)
			go func() {
				conn, err := sshDial(network, addr)
				if err == nil && conn != nil {
					conn = deadlineFreeConn{Conn: conn}
				}
				ch <- res{conn, err}
			}()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case r := <-ch:
				return r.conn, r.err
			}
		}
		return dialViaProxy(ctx, base, network, addr)
	}

	return &dialBundle{
		dialer:    wrapDialTLS(dialer, tlsCfg),
		tlsConfig: tlsCfg,
		resources: resources,
	}, nil
}

func wrapDialTLS(dial func(context.Context, string, string) (net.Conn, error), cfg *tls.Config) func(context.Context, string, string) (net.Conn, error) {
	if cfg == nil {
		return dial
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		tlsCfg := cfg.Clone()
		if tlsCfg.ServerName == "" {
			host, _, splitErr := net.SplitHostPort(addr)
			if splitErr != nil {
				host = addr
			}
			tlsCfg.ServerName = host
		}
		return tls.Client(conn, tlsCfg), nil
	}
}

func buildProxyDialer(p *config.ProxyConfig) (proxy.Dialer, error) {
	var auth *proxy.Auth
	if p.Username != "" {
		auth = &proxy.Auth{
			User:     p.Username,
			Password: p.Password,
		}
	}
	switch strings.ToLower(p.Type) {
	case "socks5":
		d, err := proxy.SOCKS5("tcp", p.Addr, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5 proxy: %w", err)
		}
		return d, nil
	case "http":
		u := &url.URL{Scheme: "http", Host: p.Addr}
		if p.Username != "" {
			u.User = url.UserPassword(p.Username, p.Password)
		}
		d, err := proxy.FromURL(u, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("http proxy: %w", err)
		}
		return d, nil
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", p.Type)
	}
}

func dialViaProxy(ctx context.Context, base proxy.Dialer, network, addr string) (net.Conn, error) {
	if base == proxy.Direct {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	type res struct {
		conn net.Conn
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		conn, err := base.Dial(network, addr)
		ch <- res{conn, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.conn, r.err
	}
}

func connectSSH(tunnel *config.SSHTunnel, base proxy.Dialer) (*ssh.Client, error) {
	host := strings.TrimSpace(tunnel.Host)
	if host == "" {
		return nil, fmt.Errorf("ssh host required")
	}
	if !strings.Contains(host, ":") {
		host += ":22"
	}
	if strings.TrimSpace(tunnel.User) == "" {
		return nil, fmt.Errorf("ssh user required")
	}

	conn, err := base.Dial("tcp", host)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}

	var auth []ssh.AuthMethod
	if tunnel.Password != "" {
		auth = append(auth, ssh.Password(tunnel.Password))
	}
	if keyPath := config.ExpandPath(tunnel.PrivateKey); keyPath != "" {
		signer, err := loadSSHSigner(keyPath, tunnel.PrivateKeyPassphrase)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if len(auth) == 0 {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh password or private_key required")
	}

	hostKeyCallback, err := sshHostKeyCallback(tunnel)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	sshHost := host
	if h, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		sshHost = h
	}

	cfg := &ssh.ClientConfig{
		User:            tunnel.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, sshHost, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	return ssh.NewClient(clientConn, chans, reqs), nil
}

func sshHostKeyCallback(tunnel *config.SSHTunnel) (ssh.HostKeyCallback, error) {
	if path := config.ExpandPath(tunnel.KnownHosts); path != "" {
		return knownhosts.New(path)
	}
	if tunnel.InsecureSkipVerify {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	return nil, fmt.Errorf("ssh known_hosts or ssh_tunnel.insecure_skip_verify required")
}

func loadSSHSigner(path, passphrase string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ssh key: %w", err)
	}
	if passphrase != "" {
		key, err := ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("parse ssh key: %w", err)
		}
		return key, nil
	}
	key, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse ssh key: %w", err)
	}
	return key, nil
}

func buildTLSConfig(p config.Profile) (*tls.Config, error) {
	if p.TLS == nil || !p.TLS.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		InsecureSkipVerify: p.TLS.InsecureSkipVerify,
		ServerName:         strings.TrimSpace(p.TLS.ServerName),
	}
	if caPath := config.ExpandPath(p.TLS.CAFile); caPath != "" {
		data, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read ca cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("invalid ca cert")
		}
		cfg.RootCAs = pool
	}
	certPath := config.ExpandPath(p.TLS.CertFile)
	keyPath := config.ExpandPath(p.TLS.KeyFile)
	if certPath != "" || keyPath != "" {
		if certPath == "" || keyPath == "" {
			return nil, fmt.Errorf("tls cert and key must both be set")
		}
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if cfg.ServerName == "" {
		cfg.ServerName = hostFromProfile(p)
	}
	return cfg, nil
}

func hostFromProfile(p config.Profile) string {
	addr := firstAddr(p)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func firstAddr(p config.Profile) string {
	if len(p.Addrs) > 0 {
		return p.Addrs[0]
	}
	return p.Addr
}

func applyDial(bundle *dialBundle, opts interface{}) {
	if bundle == nil {
		return
	}
	useDialer := bundle.dialer != nil
	handledTLS := useDialer && bundle.tlsConfig != nil
	switch o := opts.(type) {
	case *redis.Options:
		if useDialer {
			o.Dialer = bundle.dialer
		}
		if handledTLS {
			o.TLSConfig = nil
		} else {
			o.TLSConfig = bundle.tlsConfig
		}
	case *redis.ClusterOptions:
		if useDialer {
			o.Dialer = bundle.dialer
		}
		if handledTLS {
			o.TLSConfig = nil
		} else {
			o.TLSConfig = bundle.tlsConfig
		}
	case *redis.FailoverOptions:
		if useDialer {
			o.Dialer = bundle.dialer
		}
		if handledTLS {
			o.TLSConfig = nil
		} else {
			o.TLSConfig = bundle.tlsConfig
		}
	}
}
