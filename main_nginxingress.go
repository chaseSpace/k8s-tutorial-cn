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
		io.WriteString(w, fmt.Sprintf("[v3] Hello, Kubernetes!, this is ingress test, host:%s\n", host))
	})
	http.ListenAndServe(":3000", nil)
}
