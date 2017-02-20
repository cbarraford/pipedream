package main

import (
	"flag"
	"log"

	"pipedream/config"
	"pipedream/endpoints"
	"pipedream/providers/docker"
)

var configFile string

func init() {
	flag.StringVar(&configFile, "config", "", "Configuration file location")
	flag.Parse()

	if configFile == "" {
		log.Fatal("--config must be defined")
	}
}

func main() {
	// load configuration file
	conf := config.ReadConfig(configFile)
	log.Printf("Config: %+v", conf)

	// TODO: make this configurable (pick own provider)
	provider, err := docker.NewProvider(conf)
	if err != nil {
		panic(err)
	}

	r := endpoints.NewHandler(provider)
	r.Run() // listen and serve on 0.0.0.0:8080
}
