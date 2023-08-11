package main

import (
	"net"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Backend struct {
	url   *url.URL
	mu    sync.RWMutex
	alive bool
	proxy httputil.ReverseProxy
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
		log.Infof("%s is down!\n", b.url.String())
		return
	}
	defer conn.Close()
	b.SetAlive(true)
	log.Infof("%s is alive!\n", b.url.String())
}
