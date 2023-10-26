package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		io.WriteString(w, fmt.Sprintf("[v3] Hello, Kubernetes!, From host: %s\n", host))
	})
	http.ListenAndServe(":3000", nil)
}
