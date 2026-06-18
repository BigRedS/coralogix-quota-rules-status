// Command webui is a small localhost server to run quota-rules-status from a browser.
//
// Unlike a long scan, a quota-rules-status report is three quick API calls, so this
// is fully synchronous: POST /api/run does the work and returns the report as
// JSON. There are no background jobs, temp files, or downloads — which also
// means the server never writes to disk, so it runs happily with --read-only.
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"strings"

	"coralogix-quota-rules-status/internal/quotarules"
)

//go:embed index.html
var indexHTML []byte

func main() {
	listen := flag.String("listen", "localhost:8765", "HTTP listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", serveIndex())
	mux.Handle("GET /api/regions", http.HandlerFunc(handleRegions))
	mux.Handle("POST /api/run", http.HandlerFunc(handleRun))

	log.Printf("quota-rules-status webui listening on http://%s", *listen)
	if err := http.ListenAndServe(*listen, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func serveIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
}

func handleRegions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"regions": quotarules.SortedRegions(),
	})
}

type runRequest struct {
	Region string `json:"region"`
	APIKey string `json:"api_key"`
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var req runRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	req.Region = strings.TrimSpace(req.Region)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.APIKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "api_key required"})
		return
	}

	host, err := quotarules.HostForRegion(req.Region)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	client := quotarules.NewClient(host, req.APIKey)
	report, err := quotarules.FetchReport(client)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
