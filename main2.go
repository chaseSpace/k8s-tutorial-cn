package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "[v2] Hello, Kubernetes!")
	})

	log.Printf("access http://localhost:3000\n")
	panic(http.ListenAndServe(":3000", nil))
}
