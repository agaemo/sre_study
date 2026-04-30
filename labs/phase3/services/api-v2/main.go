package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const version = "v2"

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		json.NewEncoder(w).Encode(map[string]string{
			"version": version,
			"message": "Hello from v2 - new feature!",
		})
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{Addr: ":8080", Handler: mux}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
		sig := <-quit
		fmt.Printf("[%s] received signal %s, shutting down gracefully...\n", version, sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			fmt.Printf("[%s] shutdown error: %v\n", version, err)
		}
	}()

	fmt.Printf("api %s listening on :8080\n", version)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("server error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[%s] shutdown complete\n", version)
}
