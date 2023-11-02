package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	readFile := func(f string) string {
		nbytes, err := os.ReadFile("/etc/configmap_vol/" + f)
		if err != nil {
			panic(err)
		}
		return string(nbytes)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		dbURL := os.Getenv("DB_URL")
		io.WriteString(w, fmt.Sprintf("[v4] Hello, Kubernetes! From host: %s\n"+
			"Get Database Connect URL: %s\n"+
			"app-config.json:%s", host, dbURL, readFile("app-config.json")))
	})
	http.ListenAndServe(":3000", nil)
}
