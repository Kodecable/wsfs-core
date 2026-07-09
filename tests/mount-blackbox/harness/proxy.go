package harness

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
)

type tcpProxy struct {
	listener net.Listener
	target   string

	lock  sync.Mutex
	conns map[*proxyConn]struct{}
}

type proxyConn struct {
	client net.Conn
	server net.Conn
}

func startTCPProxy(rawURL string) (*tcpProxy, string, error) {
	targetURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("parse endpoint for proxy: %w", err)
	}

	switch strings.ToLower(targetURL.Scheme) {
	case "wsfs", "wss", "ws", "http", "https", "tcp", "":
	default:
		return nil, "", Skip("session resume case requires a TCP endpoint")
	}

	if targetURL.Host == "" {
		return nil, "", fmt.Errorf("endpoint host is empty")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("listen proxy: %w", err)
	}

	p := &tcpProxy{
		listener: ln,
		target:   targetURL.Host,
		conns:    map[*proxyConn]struct{}{},
	}
	go p.serve()

	targetURL.Host = ln.Addr().String()
	return p, targetURL.String(), nil
}

func (p *tcpProxy) serve() {
	for {
		client, err := p.listener.Accept()
		if err != nil {
			return
		}

		server, err := net.Dial("tcp", p.target)
		if err != nil {
			_ = client.Close()
			continue
		}

		pc := &proxyConn{client: client, server: server}
		p.track(pc)
		go p.pipe(pc, client, server)
		go p.pipe(pc, server, client)
	}
}

func (p *tcpProxy) pipe(pc *proxyConn, dst io.WriteCloser, src io.ReadCloser) {
	_, _ = io.Copy(dst, src)
	_ = dst.Close()
	_ = src.Close()
	p.untrack(pc)
}

func (p *tcpProxy) track(pc *proxyConn) {
	p.lock.Lock()
	p.conns[pc] = struct{}{}
	p.lock.Unlock()
}

func (p *tcpProxy) untrack(pc *proxyConn) {
	p.lock.Lock()
	delete(p.conns, pc)
	p.lock.Unlock()
}

func (p *tcpProxy) CloseActiveConnections() error {
	p.lock.Lock()
	conns := make([]*proxyConn, 0, len(p.conns))
	for pc := range p.conns {
		conns = append(conns, pc)
	}
	p.lock.Unlock()

	for _, pc := range conns {
		_ = pc.client.Close()
		_ = pc.server.Close()
	}
	return nil
}

func (p *tcpProxy) Close() error {
	_ = p.CloseActiveConnections()
	if p.listener == nil {
		return nil
	}
	return p.listener.Close()
}
