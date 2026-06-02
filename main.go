package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

type Backend struct {
	url   *url.URL
	alive atomic.Bool
	proxy *httputil.ReverseProxy
}

type Pool struct {
	backends []*Backend
	current  atomic.Uint32
}

func (p *Pool) next() *Backend {
	n := p.current.Add(1)
	l := uint32(len(p.backends))
	for i := uint32(0); i < l; i++ {
		b := p.backends[(n+i)%l]
		if b.alive.Load() {
			return b
		}
	}
	return nil
}

func (p *Pool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b := p.next()
	if b == nil {
		http.Error(w, "503 Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	b.proxy.ServeHTTP(w, r)
}

func (p *Pool) healthCheck() {
	t := time.NewTicker(5 * time.Second)
	for range t.C {
		for _, b := range p.backends {
			conn, err := net.DialTimeout("tcp", b.url.Host, 2*time.Second)
			alive := err == nil
			b.alive.Store(alive)
			if alive {
				conn.Close()
			}
			status := "UP"
			if !alive {
				status = "DOWN"
			}
			log.Printf("health %s [%s]", b.url.Host, status)
		}
	}
}

func main() {
	addrs := []string{
		"http://127.0.0.1:8081",
		"http://127.0.0.1:8082",
		"http://127.0.0.1:8083",
	}

	pool := &Pool{}

	for _, addr := range addrs {
		u, err := url.Parse(addr)
		if err != nil {
			log.Fatalf("invalid url %s: %v", addr, err)
		}

		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("upstream %s error: %v", u.Host, err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		}

		b := &Backend{url: u, proxy: proxy}
		b.alive.Store(true)
		pool.backends = append(pool.backends, b)
	}

	go pool.healthCheck()

	srv := &http.Server{
		Addr:         "127.0.0.1:8080",
		Handler:      pool,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("balancer listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
