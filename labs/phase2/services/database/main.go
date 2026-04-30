package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// データベース役のサービス。意図的に遅延・障害を再現できる。
func main() {
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		// クエリに50〜150msかかる想定
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
		fmt.Fprintf(w, `{"result": "ok", "rows": 42}`)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	fmt.Println("database service listening on :8082")
	http.ListenAndServe(":8082", nil)
}
