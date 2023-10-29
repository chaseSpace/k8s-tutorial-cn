package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	http.HandleFunc("/now_time", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, fmt.Sprintf("[v3] Hello, Kubernetes!, now time: %s\n", time.Now()))
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		io.WriteString(w, fmt.Sprintf("[v3] Hello, Kubernetes!, From host: %s\n", host))
	})
	http.ListenAndServe(":3000", nil)
}
