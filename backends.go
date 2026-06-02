package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

func serve(port int, wg *sync.WaitGroup) {
	defer wg.Done()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("backend :%d <- %s %s", port, r.Method, r.URL.Path)
		fmt.Fprintf(w, "backend port %d\n", port)
	})
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("backend listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("backend :%d failed: %v", port, err)
	}
}

func main() {
	ports := []int{8081, 8082, 8083}
	var wg sync.WaitGroup
	for _, p := range ports {
		wg.Add(1)
		go serve(p, &wg)
	}
	wg.Wait()
}
