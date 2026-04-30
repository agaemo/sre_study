package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

var (
	backendURL = getEnv("BACKEND_URL", "http://backend:8081")
	client     = &http.Client{Timeout: 3 * time.Second}
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		resp, err := client.Get(backendURL + "/api/data")
		duration := time.Since(start).Milliseconds()

		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":       err.Error(),
				"duration_ms": duration,
			})
			return
		}
		defer resp.Body.Close()

		var body map[string]string
		json.NewDecoder(resp.Body).Decode(&body)
		body["duration_ms"] = fmt.Sprintf("%dms", duration)
		body["status"] = fmt.Sprintf("%d", resp.StatusCode)

		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(body)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	fmt.Println("frontend service listening on :8080")
	http.ListenAndServe(":8080", nil)
}
