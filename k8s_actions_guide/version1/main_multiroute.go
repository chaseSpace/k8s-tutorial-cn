package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"net/http"
	"os"
)

type Config struct {
	Routes map[string]string `yaml:"routes"`
}

func main() {
	cfg, ok := loadConfig()
	if !ok {
		return
	}
	for route, resp := range cfg.Routes {
		log.Printf("Load path:%s\n", route)
		_route := route
		_resp := resp // 避免闭包问题
		http.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello, You are at %s, Got: %s", _route, _resp)
		})
	}
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