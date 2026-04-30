package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// CircuitBreaker はサーキットブレーカーの状態を管理する
type CircuitBreaker struct {
	failures  atomic.Int64
	openUntil atomic.Int64 // Unix nanoseconds
	threshold int64
	timeout   time.Duration
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{threshold: int64(threshold), timeout: timeout}
}

func (cb *CircuitBreaker) isOpen() bool {
	until := cb.openUntil.Load()
	if until == 0 {
		return false
	}
	if time.Now().UnixNano() < until {
		return true
	}
	// タイムアウト経過でハーフオープンへ
	cb.openUntil.Store(0)
	cb.failures.Store(0)
	return false
}

func (cb *CircuitBreaker) recordFailure() {
	count := cb.failures.Add(1)
	if count >= cb.threshold {
		// サーキットをオープンにする
		cb.openUntil.Store(time.Now().Add(cb.timeout).UnixNano())
		cb.failures.Store(0)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.failures.Store(0)
}

func (cb *CircuitBreaker) State() string {
	if cb.isOpen() {
		return "open"
	}
	return "closed"
}

var (
	dbURL          = getEnv("DB_URL", "http://database:8082")
	useCircuitBreaker = getEnv("USE_CIRCUIT_BREAKER", "false") == "true"
	cb             = NewCircuitBreaker(3, 10*time.Second)
	client         = &http.Client{Timeout: 2 * time.Second}
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func queryDB() error {
	resp, err := client.Get(dbURL + "/query")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("db returned %d", resp.StatusCode)
	}
	return nil
}

func main() {
	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		state := "n/a"
		if useCircuitBreaker {
			state = cb.State()
			if cb.isOpen() {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{
					"error":          "circuit breaker open",
					"circuit_breaker": state,
				})
				return
			}
		}

		if err := queryDB(); err != nil {
			if useCircuitBreaker {
				cb.recordFailure()
				state = cb.State()
			}
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{
				"error":          err.Error(),
				"circuit_breaker": state,
			})
			return
		}

		if useCircuitBreaker {
			cb.recordSuccess()
			state = cb.State()
		}

		json.NewEncoder(w).Encode(map[string]string{
			"data":           "hello from backend",
			"circuit_breaker": state,
		})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	fmt.Printf("backend service listening on :8081 (circuit_breaker=%v)\n", useCircuitBreaker)
	http.ListenAndServe(":8081", nil)
}

// io パッケージを使用するために追加
var _ = io.Discard
