package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

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

	pool := NewPool()

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

	go pool.HealthCheck(time.Minute)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
