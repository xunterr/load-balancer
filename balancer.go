package main

import (
	"context"
	"net/http"
	"net/http/httputil"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

type contextKey int

const (
	Retry contextKey = iota
	CurrentBackend
)

var (
	currentPool *Pool
)

type Pool struct {
	backends []*Backend
	current  uint64
}

func NewPool() *Pool {
	currentPool = &Pool{
		backends: []*Backend{},
		current:  0,
	}
	return currentPool
}

func (p *Pool) AddBackend(b *Backend) {
	p.backends = append(p.backends, b)
}

func (p *Pool) NextIndex() int {
	return int(atomic.AddUint64(&p.current, 1) % uint64(len(p.backends)))
}

func (p *Pool) RoundRobin() *Backend {
	next := p.NextIndex()
	for i := next; i < len(p.backends)+next; i++ {
		idx := i % len(p.backends)
		if p.backends[idx].IsAlive() {
			p.current = uint64(i)
			return p.backends[idx]
		}
	}
	return nil
}

func (p *Pool) LeastConns() *Backend {
	max := 0
	for i := 0; i < len(p.backends); i++ {
		if p.backends[i].connections > p.backends[max].connections {
			max = i
		}
	}
	return p.backends[max]
}

func (p *Pool) HealthCheck(d time.Duration) {
	log.Infof("Healthcheck initialized with %fs periodicity", d.Seconds())
	t := time.NewTicker(d)
	for range t.C {
		for _, b := range p.backends {
			go b.HealthCheck()
		}
	}
}

func handleError(w http.ResponseWriter, req *http.Request, err error, proxy httputil.ReverseProxy) {

	retry, ok := req.Context().Value(Retry).(int)
	if !ok {
		retry = 0
	}
	b, ok := req.Context().Value(CurrentBackend).(*Backend)
	if !ok {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if retry > 3 {
		b.SetAlive(false)
		log.Warnf("[%s](%s) Marked as down after 3 attempts", req.URL.String(), b.url.Host)
		balance(w, req)
		return
	}

	select {
	case <-time.After(50 * time.Millisecond):
		ctx := context.WithValue(req.Context(), Retry, retry+1)
		proxy.ServeHTTP(w, req.WithContext(ctx))
	}
}

func balance(w http.ResponseWriter, req *http.Request) {
	b := currentPool.RoundRobin()
	if b == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		log.Errorf("Cant serve %s : No available alive servers", req.URL.String())
		return
	}
	req.Host = b.url.Host
	req.URL.Host = b.url.Host
	req.URL.Scheme = b.url.Scheme

	b.AddConn()
	ctx := context.WithValue(req.Context(), CurrentBackend, b)
	log.Infof("Serving %s -> [%s]", req.RemoteAddr, req.URL.String())
	b.proxy.ServeHTTP(w, req.WithContext(ctx))
	b.RemoveConn()
}
