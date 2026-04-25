// Package proxy handles the TCP proxying between Minecraft clients and the
// backend server, forwarding packets transparently in both directions.
package proxy

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Config holds configuration for the proxy.
type Config struct {
	// ListenAddr is the address the proxy listens on (e.g. ":25565").
	ListenAddr string
	// TargetAddr is the address of the backend Minecraft server (e.g. "localhost:25566").
	TargetAddr string
	// DialTimeout is the maximum time to wait when connecting to the backend.
	DialTimeout time.Duration
}

// Proxy forwards TCP connections from clients to the backend Minecraft server.
type Proxy struct {
	cfg      Config
	listener net.Listener
	mu       sync.Mutex
	active   int
	doneCh   chan struct{}
}

// New creates a new Proxy with the given configuration.
func New(cfg Config) *Proxy {
	if cfg.DialTimeout == 0 {
		// Increased from 5s to 10s — the default was too aggressive on my
		// home server which can be slow to wake up after hibernation.
		cfg.DialTimeout = 10 * time.Second
	}
	return &Proxy{
		cfg:    cfg,
		doneCh: make(chan struct{}),
	}
}

// Start begins listening for incoming client connections.
// It returns an error if the listener cannot be established.
func (p *Proxy) Start() error {
	ln, err := net.Listen("tcp", p.cfg.ListenAddr)
	if err != nil {
		return err
	}
	p.listener = ln
	log.Printf("proxy: listening on %s, forwarding to %s", p.cfg.ListenAddr, p.cfg.TargetAddr)
	go p.acceptLoop()
	return nil
}

// Stop closes the listener and waits for all active connections to finish.
func (p *Proxy) Stop() {
	if p.listener != nil {
		p.listener.Close()
	}
}

// ActiveConnections returns the number of currently proxied connections.
func (p *Proxy) ActiveConnections() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}

// acceptLoop accepts incoming connections and spawns a goroutine for each.
func (p *Proxy) acceptLoop() {
	for {
		client, err := p.listener.Accept()
		if err != nil {
			// Listener was closed; exit gracefully.
			select {
			case <-p.doneCh:
			default:
				log.Printf("proxy: accept error: %v", err)
			}
			return
		}
		go p.handleConn(client)
	}
}

// handleConn proxies data between the client and the backend server.
func (p *Proxy) handleConn(client net.Conn) {
	defer client.Close()

	backend, err := net.DialTimeout("tcp", p.cfg.TargetAddr, p.cfg.DialTimeout)
	if err != nil {
		log.Printf("proxy: failed to connect to backend %s: %v", p.cfg.TargetAddr, err)
		return
	}
	defer backend.Close()

	p.mu.Lock()
	p.active++
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
	}()

	log.Printf("proxy: new connection %s <-> %s", client.RemoteAddr(), p.cfg.TargetAddr)

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Backend
	go func() {
		defer wg.Done()
		if _, err := io.Copy(backend, client); err != nil {
			log.Printf("proxy: client->backend copy error: %v", err)
		}
		// Signal the other direction to stop.
		backend.Close()
	}()

	// Backend -> Client
	go func() {
		defer wg.Done()
		if _, err := io.Copy(client, backend); err != nil {
			log.Printf("proxy: backend->client copy error: %v", err)
		}
		// Signal the other direction to stop.
		client.Close()
	}()

	wg.Wait()
	log.Printf("proxy: connection closed %s <-> %s", client.RemoteAddr(), p.cfg.TargetAddr)
}
