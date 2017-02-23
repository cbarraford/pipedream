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
	repos    map[string]time.Time
	idle     time.Duration
	pulls    map[string]bool
	alwaysOn map[string][]string
	github   github.GithubService
}

func (r *LastRequest) Setup(provider providers.Provider, conf config.Config) error {
	// populate last request
	applications, err := provider.ListApps()
	if err != nil {
		return err
	}
	for _, app := range applications {
		r.AddRequest(app)
	}

	for name, repoConfig := range conf.Repository {
		r.alwaysOn[name] = repoConfig.AlwaysOn
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

func (r *LastRequest) AddPull(app apps.App) {
	key := fmt.Sprintf("%s/%s/%s", app.Org, app.Repo, app.Branch)
	r.pulls[key] = true
}

func (r *LastRequest) RemovePull(app apps.App) {
	key := fmt.Sprintf("%s/%s/%s", app.Org, app.Repo, app.Branch)
	delete(r.pulls, key)
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
				} else {
					r.RemoveRequest(app)
				}
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

	pulls, _ := r.github.ListOpenPullRequests("cbarraford", "pipedream-simple")
	for _, pull := range pulls {
		parts := strings.Split(*pull.Head.Label, ":")
		branch := parts[1]
		commit := *pull.Head.SHA
		app := apps.NewApp("cbarraford", "pipedream-simple", branch, commit)

		reserved = append(reserved, app)
	}

	for k, branches := range r.alwaysOn {
		parts := strings.Split(k, "/")
		org, repo := parts[0], parts[1]
		for _, branch := range branches {
			commit, err := r.github.GetReference(branch)
			if err != nil {
				log.Printf("Error getting git reference: %+v", err)
			}
			reserved = append(reserved, apps.NewApp(org, repo, branch, commit))
		}
	}

	filtered := make([]apps.App, 0)
	for _, a := range stale {
		shouldFilter := false
		for _, b := range reserved {
			if a.Org == b.Org && a.Repo == b.Repo && a.Commit == b.Commit {
				log.Printf("Filtered: %+v", a)
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
