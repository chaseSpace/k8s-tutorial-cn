package main

import (
	"net/http"
)

func main() {
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("server1: Hello, World!"))
		})
		println("listening on :3100")
		http.ListenAndServe(":3100", nil)
	}()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("server2: Hello, World!"))
	})
	println("listening on :3200")
	http.ListenAndServe(":3200", h)
}
