package endpoints

import (
	"fmt"
	"log"
	"pipedream/config"
	"pipedream/providers"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type LastRequest struct {
	repos map[string]time.Time
	idle  time.Duration
	conf  config.Config
}

func (r *LastRequest) keyify(org, repo, branch string) string {
	return fmt.Sprintf("%s %s %s", org, repo, branch)
}

func (r *LastRequest) AddRequest(org, repo, branch string) {
	key := r.keyify(org, repo, branch)
	r.repos[key] = time.Now()
}

func (r *LastRequest) Remove(org, repo, branch string) {
	delete(r.repos, r.keyify(org, repo, branch))
}

func (r *LastRequest) StartTicker(provider providers.Provider) {
	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for _ = range ticker.C {
			stale := r.GetStaleApps()
			for _, app := range stale {
				org, repo, branch := app[0], app[1], app[2]
				err := provider.Stop(org, repo, branch)
				if err != nil {
					log.Printf("Error stopping app: %+v", err)
				} else {
					r.Remove(org, repo, branch)
				}
			}
		}
	}()
}

func (r *LastRequest) GetStaleApps() [][]string {
	stale := make([][]string, 0)
	for repo, lastRequest := range r.repos {
		// skip apps that are "alwaysOn"
		parts := strings.Split(repo, " ")
		org, repo, branch := parts[0], parts[1], parts[2]
		repoConf, ok := r.conf.GetRepo(org, repo)
		if ok {
			on := false
			for _, alwaysOn := range repoConf.AlwaysOn {
				if alwaysOn == branch {
					on = true
				}
			}
			if on {
				continue
			}
		}

		duration := time.Since(lastRequest)
		if duration > r.idle {
			stale = append(stale, parts)
		}
	}
	return stale
}

func (r LastRequest) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		org := c.Param("org")
		repo := c.Param("repo")
		branch := c.Param("branch")
		if org != "" && repo != "" && branch != "" {
			r.AddRequest(org, repo, branch)
		}
	}
}
