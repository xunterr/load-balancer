package main

import (
	"net"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

type Backend struct {
	url         *url.URL
	mu          sync.RWMutex
	connections uint64
	alive       bool
	proxy       httputil.ReverseProxy
}

func (b *Backend) AddConn() {
	atomic.AddUint64(&b.connections, 1)
}

func (b *Backend) RemoveConn() {
	atomic.AddUint64(&b.connections, ^uint64(0))
}

func (b *Backend) IsAlive() (alive bool) {
	b.mu.Lock()
	alive = b.alive
	b.mu.Unlock()
	return
}

func (b *Backend) SetAlive(alive bool) {
	b.mu.RLock()
	b.alive = alive
	b.mu.RUnlock()
}

func (b *Backend) HealthCheck() {
	conn, err := net.DialTimeout("tcp", b.url.Host, 5*time.Second)
	if err != nil {
		b.SetAlive(false)
		log.Infof("%s is down: %s", b.url.String(), err.Error())
		return
	}
	defer conn.Close()
	b.SetAlive(true)
	log.Infof("%s is alive!", b.url.String())
}
