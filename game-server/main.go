package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type response struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Note    string `json:"note,omitempty"`
}

func jsonResponse(w http.ResponseWriter, code int, v response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, http.StatusOK, response{Service: "game-server", Status: "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, http.StatusServiceUnavailable, response{Service: "game-server", Status: "scaffold-only", Note: "Database/Redis and room services not wired yet"})
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, http.StatusNotImplemented, response{Service: "game-server", Status: "not-implemented", Note: "WSS JSON/binary room protocol is not yet implemented"})
	})
	// IMPORTANT: no server-side movement, authentication or player data exists yet.
	addr := os.Getenv("GAME_SERVER_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Game Server scaffold listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
