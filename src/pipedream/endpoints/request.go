package endpoints

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type LastRequest struct {
	repos map[string]time.Time
	idle  time.Duration
}

func (r *LastRequest) keyify(org, repo, branch string) string {
	return fmt.Sprintf("%s %s %s", org, repo, branch)
}

func (r *LastRequest) AddRequest(org, repo, branch string) {
	key := r.keyify(org, repo, branch)
	r.repos[key] = time.Now()
}

func (r *LastRequest) GetStaleApps() [][]string {
	stale := make([][]string, 0)
	for repo, lastRequest := range r.repos {
		duration := time.Since(lastRequest)
		if duration > r.idle {
			stale = append(stale, strings.Split(repo, " "))
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
