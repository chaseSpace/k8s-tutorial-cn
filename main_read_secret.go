package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	readFile := func(f string) string {
		nbytes, err := os.ReadFile("/etc/secret_vol/" + f)
		if err != nil {
			panic(err)
		}
		return string(nbytes)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		io.WriteString(w, fmt.Sprintf("[v4] Hello, Kubernetes! From host: %s, Get Database Passwd: %s\n"+
			"some.txt:%s\ncert.key:%s\nconfig.yaml:%s",
			host, os.Getenv("DB_PASSWD"), readFile("some.txt"), readFile("cert.key"), readFile("config.yaml")))
	})
	http.ListenAndServe(":3000", nil)
}
