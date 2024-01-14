package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	go func() {
		i := 0
		for {
			i++
			time.Sleep(time.Second)
			// 实际项目中最好在日志中使用毫秒级的时间，这里仅精确到秒
			fmt.Println(fmt.Sprintf(`{"time": "%s", "number": %d, "field1":"abcdefghijklmn","field2":"0123456789","field3":"Golang","field4":"Kubernetes"}`, time.Now().Format(time.DateTime), i))
		}
	}()
	http.ListenAndServe(":3000", nil)
}
