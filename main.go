package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
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
	l := uint32(len(p.backends))
	if l == 0 {
		return nil
	}

	n := p.current.Add(1)

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
		http.Error(w, "503 Service Unavailable (All backends are down)", http.StatusServiceUnavailable)
		return
	}
	b.proxy.ServeHTTP(w, r)
}

func (p *Pool) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
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
				log.Printf("[HEALTH] %s is %s", b.url.Host, status)
			}
		case <-ctx.Done():
			log.Println("[HEALTH] Background health check stopped")
			return
		}
	}
}

func main() {
	log.Println("[SYSTEM] Starting L7 Load Balancer infrastructure...")

	StartTestBackends()

	addrs := []string{
		"http://127.0.0.1:8081",
		"http://127.0.0.1:8082",
		"http://127.0.0.1:8083",
	}

	pool := &Pool{}

	for _, addr := range addrs {
		u, err := url.Parse(addr)
		if err != nil {
			log.Fatalf("[FATAL] Invalid target URL %s: %v", addr, err)
		}

		proxy := httputil.NewSingleHostReverseProxy(u)
		
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[PROXY ERROR] Upstream %s failed: %v", u.Host, err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		}

		b := &Backend{url: u, proxy: proxy}
		b.alive.Store(true)
		pool.backends = append(pool.backends, b)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pool.healthCheck(ctx)

	srv := &http.Server{
		Addr:         "127.0.0.1:8080",
		Handler:      pool,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		
		log.Println("[SYSTEM] Shutting down gracefully...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("[ERROR] Server forced to shutdown: %v", err)
		}
	}()

	log.Printf("[SYSTEM] Balancer is core-ready and listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[FATAL] Server error: %v", err)
	}
	
	log.Println("[SYSTEM] Balancer successfully stopped.")
}
