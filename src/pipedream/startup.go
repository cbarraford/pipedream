package main

import (
	"flag"
	"log"

	"pipedream/config"
	"pipedream/endpoints"
	"pipedream/providers/docker"
	"pipedream/services/github"
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

	// TODO: make this configurable (pick own provider)
	provider, err := docker.NewProvider(conf)
	if err != nil {
		log.Fatal(err)
	}

	githubClient := github.NewClient(
		conf.Github.Token,
		conf.General.ServerAddress,
		conf.Github.Secret,
	)
	if err := githubClient.Setup(); err != nil {
		log.Fatal(err)
	}

	r := endpoints.NewHandler(conf, provider, githubClient)
	r.Run() // listen and serve on 0.0.0.0:8080
}
