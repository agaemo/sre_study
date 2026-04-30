package main

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "status"},
	)

	// SLIの計測に使うヒストグラム（パーセンタイル計算ができる）
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"path"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration)
}

func instrument(path string, handler func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		handler(rw, r)
		duration := time.Since(start).Seconds()

		status := "2xx"
		if rw.status >= 500 {
			status = "5xx"
		} else if rw.status >= 400 {
			status = "4xx"
		}

		requestsTotal.WithLabelValues(path, status).Inc()
		requestDuration.WithLabelValues(path).Observe(duration)
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func main() {
	http.Handle("/metrics", promhttp.Handler())

	// 正常なエンドポイント（5〜20ms）
	http.HandleFunc("/api/hello", instrument("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(5+rand.Intn(15)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "hello"}`))
	}))

	// 意図的に遅いエンドポイント（200〜800ms）→ P99を悪化させる
	http.HandleFunc("/api/slow", instrument("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(200+rand.Intn(600)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "slow response"}`))
	}))

	// エラーを返すエンドポイント → エラーバジェットを消費させる
	http.HandleFunc("/api/error", instrument("/api/error", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))

	http.ListenAndServe(":8080", nil)
}
