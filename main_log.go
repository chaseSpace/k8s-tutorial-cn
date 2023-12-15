package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	go func() {
		i := 0
		for {
			i++
			time.Sleep(time.Second)
			log.Println("log test", i)
		}
	}()
	http.ListenAndServe(":3000", nil)
}
