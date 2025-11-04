package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Simple HTTP receiver for Terraform state PUT/GET and naive lock/unlock.
//
// Endpoints:
//   PUT  /states/{module}/{workspace}.tfstate   -> writes state file under ./data/{module}/{workspace}.tfstate
//   GET  /states/{module}/{workspace}.tfstate   -> reads the state file
//   POST /states/{module}/{workspace}/lock      -> creates ./data/{module}/{workspace}.lock
//   POST /states/{module}/{workspace}/unlock    -> removes lock file
//
// Start:
//   go run examples/http-receiver/main.go
//
// Example export target URL for kh:
//   --url http://localhost:8080/states/{module}/{workspace}.tfstate

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/states/", handleStates)
	addr := ":8080"
	log.Printf("HTTP receiver listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleStates(w http.ResponseWriter, r *http.Request) {
	// Normalize path like /states/{module}/{workspace}.tfstate or /states/{module}/{workspace}/lock
	p := strings.TrimPrefix(r.URL.Path, "/states/")
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	module := parts[0]
	tail := strings.Join(parts[1:], "/")

	// Lock/Unlock endpoints
	if strings.HasSuffix(tail, "/lock") && r.Method == http.MethodPost {
		lockPath := filepath.Join("data", module, strings.TrimSuffix(strings.TrimSuffix(tail, "/lock"), "/")) + ".lock"
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(lockPath, []byte("locked"), 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	if strings.HasSuffix(tail, "/unlock") && r.Method == http.MethodPost {
		lockPath := filepath.Join("data", module, strings.TrimSuffix(strings.TrimSuffix(tail, "/unlock"), "/")) + ".lock"
		_ = os.Remove(lockPath)
		w.WriteHeader(http.StatusOK)
		return
	}

	// State file path
	if !strings.HasSuffix(tail, ".tfstate") {
		http.Error(w, "expected .tfstate path", http.StatusBadRequest)
		return
	}
	// Extract workspace from filename or subdir
	base := filepath.Base(tail)
	workspace := strings.TrimSuffix(base, ".tfstate")
	fsPath := filepath.Join("data", module, workspace+".tfstate")

	switch r.Method {
	case http.MethodPut:
		if err := os.MkdirAll(filepath.Dir(fsPath), 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(fsPath, b, 0o600); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sum := sha256.Sum256(b)
		out := map[string]any{
			"url":      fmt.Sprintf("%s://%s%s", scheme(r), r.Host, r.URL.Path),
			"size":     len(b),
			"checksum": hex.EncodeToString(sum[:]),
		}
		writeJSON(w, out)
	case http.MethodGet:
		b, err := os.ReadFile(fsPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.terraform.state+json;version=4")
		_, _ = w.Write(b)
	default:
		w.Header().Set("Allow", "PUT, GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if r.Header.Get("X-Forwarded-Proto") != "" {
		return r.Header.Get("X-Forwarded-Proto")
	}
	return "http"
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}
