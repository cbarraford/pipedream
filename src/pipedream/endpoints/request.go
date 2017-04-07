package endpoints

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pipedream/apps"
	"pipedream/config"
	"pipedream/providers"
	"pipedream/services/github"
)

type LastRequest struct {
	repos  map[string]time.Time
	idle   time.Duration
	pulls  map[string]bool
	github github.GithubService
	config config.Config
}

func (r *LastRequest) Setup(provider providers.Provider) error {
	// populate last request
	applications, err := provider.ListApps()
	if err != nil {
		return err
	}
	for _, app := range applications {
		r.AddRequest(app)
	}

	r.StartTicker(provider)

	return nil
}

func (r *LastRequest) AddRequest(app apps.App) {
	key := app.String()
	r.repos[key] = time.Now()
}

func (r *LastRequest) RemoveRequest(app apps.App) {
	delete(r.repos, app.String())
}

func (r *LastRequest) Get(app apps.App) time.Time {
	key := app.String()
	return r.repos[key]
}

func (r *LastRequest) StartTicker(provider providers.Provider) {
	ticker := time.NewTicker(time.Second * 10)
	go func() {
		for _ = range ticker.C {
			stale := r.GetStaleApps()
			for _, app := range stale {
				err := provider.Stop(app)
				if err != nil {
					log.Printf("Error stopping app: %+v", err)
				}
				r.RemoveRequest(app)
			}
		}
	}()
}

func (r *LastRequest) GetStaleApps() []apps.App {
	stale := make([]apps.App, 0)
	for repo, lastRequest := range r.repos {
		parts := strings.Split(repo, ".")
		org, repo, commit := parts[0], parts[1], parts[3]
		app := apps.NewApp(org, repo, "", commit)

		duration := time.Since(lastRequest)
		if duration > r.idle {
			stale = append(stale, app)
		}
	}
	return r.filterReserved(stale)
}

func (r *LastRequest) filterReserved(stale []apps.App) []apps.App {
	if len(stale) == 0 {
		return nil
	}
	reserved := make([]apps.App, 0)

	for name, _ := range r.config.Repository {
		parts := strings.Split(name, "/")
		org, repo := parts[0], parts[1]
		pulls, _ := r.github.ListOpenPullRequests(org, repo)
		for _, pull := range pulls {
			parts := strings.Split(*pull.Head.Label, ":")
			branch := parts[1]
			commit := *pull.Head.SHA
			app := apps.NewApp(org, repo, branch, commit)

			reserved = append(reserved, app)
		}
	}

	filtered := make([]apps.App, 0)
	for _, a := range stale {
		astr := strings.ToLower(fmt.Sprintf("%s.%s.%s", a.Org, a.Repo, a.Commit))
		shouldFilter := false
		for _, b := range reserved {
			bstr := strings.ToLower(fmt.Sprintf("%s.%s.%s", b.Org, b.Repo, b.Commit))
			if astr == bstr {
				shouldFilter = true
			}
		}
		if !shouldFilter {
			filtered = append(filtered, a)
		}
	}

	return filtered
}

func (r LastRequest) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		org := c.Param("org")
		repo := c.Param("repo")
		branch := c.Param("branch")
		commit := c.Param("commit")
		if org != "" && repo != "" && commit != "" {
			app := apps.NewApp(org, repo, branch, commit)
			r.AddRequest(app)
		}
	}
}
