package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"net/http"
	"os"
	"time"
)

type RouteConf struct {
	Response string        `yaml:"response"`
	Duration time.Duration `yaml:"duration"`
}

type Config struct {
	Routes map[string]*RouteConf `yaml:"routes"`
}

func main() {
	cfg, ok := loadConfig()
	if !ok {
		return
	}
	version := os.Getenv("VERSION")
	for route, rconf := range cfg.Routes {
		log.Printf("Load path:%s, conf: %+v\n", route, *rconf)
		_route := route
		_rconf := *rconf
		http.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(time.Second * _rconf.Duration)
			fmt.Fprintf(w, "[%s] ", version)
			fmt.Fprintf(w, "Hello, You are at %s, Got: %s", _route, _rconf.Response)
		})
	}

	// 模拟连接数据库操作
	http.HandleFunc("/connect_db", func(w http.ResponseWriter, r *http.Request) {
		dbpass := os.Getenv("DB_PASS")

		if dbpass == "" { // 是否读取到配置
			fmt.Fprintf(w, "Sorry, no db password provided!")
			return
		} else if dbpass != "pass123" { // 验证密码
			fmt.Fprintf(w, "Sorry, wrong db password provided!")
		}

		// 连接成功
		fmt.Fprintf(w, "Hello, You are connected database successfully!")
	})

	http.HandleFunc("/get_ip", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "[%s] ", version)
		fmt.Fprintf(w, "Hello, Your ip is %s", os.Getenv("POD_IP"))
	})

	// 熔断测试
	http.HandleFunc("/test_circuit_breaker", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprintf(w, "[%s] test_circuit_breaker returned 500\n", version)
	})

	log.Printf("Listening on http://localhost:3000\n")
	panic(http.ListenAndServe(":3000", nil))
}

func loadConfig() (cfg Config, ok bool) {
	cfgFile := flag.String("config", "", "config file")
	flag.Parse()
	if *cfgFile == "" {
		fmt.Println("No config file specified")
		return
	}
	data, err := os.ReadFile(*cfgFile)
	if err != nil {
		fmt.Println("Error reading YAML file:", err)
		return
	}
	println(111, string(data))
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		fmt.Println("Error unmarshalling YAML:", err)
		return
	}
	fmt.Printf("222 %+v\n", cfg)
	return cfg, true
}
