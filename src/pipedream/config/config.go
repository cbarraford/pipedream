package config

import (
	"log"
	"os"
	"time"

	gcfg "gopkg.in/gcfg.v1"
)

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Repo struct {
	DefaultBranch string
	AlwaysOn      []string
}

type Config struct {
	General struct {
		IdleShutdown duration
		GithubToken  string
	}
	Repository map[string]*Repo
}

// Reads info from config file
func ReadConfig(configFile string) Config {
	_, err := os.Stat(configFile)
	if err != nil {
		log.Fatal("Config file is missing: ", configFile)
	}

	var config Config
	if err := gcfg.ReadFileInto(&config, configFile); err != nil {
		log.Fatal(err)
	}

	return config
}
