package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	host, _ := os.Hostname()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ingress/123":
			io.WriteString(w, fmt.Sprintf("[v3] Hello, Kubernetes!, path:%s\n", r.URL.Path))
		case "/hello":
			io.WriteString(w, fmt.Sprintf("[v3] Hello, Kubernetes!, host:%s\n", host))
		default:
			w.WriteHeader(404)
			w.Write([]byte(r.URL.Path + " is not found, 404"))
		}
	})
	http.ListenAndServe(":3000", nil)
}
