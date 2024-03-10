package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	// 执行调用
	rsp, err := http.Get("http://go-multiroute:3000/route1")
	if err != nil {
		log.Fatalln("Call err: " + err.Error())
	}

	text, _ := io.ReadAll(rsp.Body)

	// 打印响应
	log.Printf("status:%d text: %s\n", rsp.StatusCode, text)
	select {}
}
