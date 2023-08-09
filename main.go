package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	pool *Pool
)

type contextKey int

const (
	Retry contextKey = iota
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

type Pool struct {
	backends []*Backend
	current  uint64
}

func (p *Pool) AddBackend(b *Backend) {
	p.backends = append(p.backends, b)
}

func (p *Pool) GetBackend(url *url.URL) *Backend {
	for _, b := range p.backends {
		if b.url.Host == url.Host {
			return b
		}
	}
	return nil
}

func (p *Pool) NextIndex() int {
	return int(atomic.AddUint64(&p.current, 1) % uint64(len(p.backends)))
}

func (p *Pool) NextBackend() *Backend {
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

func handleError(w http.ResponseWriter, req *http.Request, err error, proxy httputil.ReverseProxy) {
	retry, ok := req.Context().Value(Retry).(int)
	if !ok {
		retry = 0
	}
	b := pool.GetBackend(req.URL)
	if b == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if retry > 3 {
		b.SetAlive(false)
		log.Warnf("[%s](%s) Marked as down after 3 attempts\n", b.url.String(), b.url.Host)
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
	b := pool.NextBackend()
	if b == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		log.Errorf("Cant serve %s : No available alive servers\n", req.URL.String())
		return
	}
	req.Host = b.url.Host
	req.URL.Host = b.url.Host
	req.URL.Scheme = b.url.Scheme
	log.Infof("Serving %s -> [%s]\n", req.RemoteAddr, req.URL.String())
	b.proxy.ServeHTTP(w, req)
}

func main() {
	serversParam := flag.String("s", "", "Servers here, separate by comas")
	flag.Parse()

	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	servers := strings.Split(*serversParam, ",")
	if len(servers) == 0 {
		return
	}

	pool = &Pool{
		backends: []*Backend{},
		current:  0,
	}
	for _, s := range servers {
		url, _ := url.Parse(s)
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			handleError(w, r, err, *proxy)
		}
		b := &Backend{url: url, alive: true, proxy: *proxy}
		pool.AddBackend(b)
	}
	pool.current = 1
	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", 8080),
		Handler: http.HandlerFunc(balance),
	}

	go func() {
		t := time.NewTicker(time.Minute)
		for range t.C {
			for _, b := range pool.backends {
				b.HealthCheck()
			}
		}
	}()

	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

}
