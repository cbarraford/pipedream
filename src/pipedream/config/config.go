package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	gcfg "gopkg.in/gcfg.v1"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Repo struct {
	DefaultRepo     bool
	DefaultBranch   string
	AlwaysOn        []string
	DockerImage     string
	HealthCheckPath string
}

type Config struct {
	General struct {
		Serve         string
		IdleShutdown  Duration
		ServerAddress string
		DockerHost    string
		DockerAddress string

		// Pass the full original request url in a specified header to application.
		// May be used by running application to generate proper fully qualified redirect URLS
		OrginalRequestHeader string
	}
	Github struct {
		Token  string
		Secret string
	}
	Repository map[string]*Repo
}

func (c Config) GetRepo(org, repo string) (*Repo, bool) {
	repoName := fmt.Sprintf("%s/%s", strings.ToLower(org), strings.ToLower(repo))
	for name, repository := range c.Repository {
		if strings.ToLower(name) == repoName {
			return repository, true
		}
	}
	return &Repo{}, false
}

func (c Config) GetDefaultRepo() (string, string) {
	for name, repository := range c.Repository {
		if repository.DefaultRepo {
			parts := strings.Split(name, "/")
			return parts[0], parts[1]
		}
	}
	return "", ""
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
