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
		jsonResponse(w, http.StatusOK, response{Service: "transaction-service", Status: "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, http.StatusServiceUnavailable, response{Service: "transaction-service", Status: "scaffold-only", Note: "Ledger and PostgreSQL transactions not wired yet"})
	})
	mux.HandleFunc("POST /api/v1/transactions", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, http.StatusNotImplemented, response{Service: "transaction-service", Status: "not-implemented", Note: "No real currency/item modifications are enabled in this scaffold"})
	})
	addr := os.Getenv("TRANSACTION_SERVICE_ADDR")
	if addr == "" {
		addr = ":8082"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Transaction Service scaffold listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
