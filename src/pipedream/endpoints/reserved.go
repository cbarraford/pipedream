package endpoints

import (
	"strings"

	"pipedream/apps"
	"pipedream/config"
	"pipedream/providers"
)

type Reserved struct {
	apps     map[string]apps.App
	pulls    map[string]bool
	provider providers.Provider
}

func NewReserved(provider providers.Provider) *Reserved {
	return &Reserved{
		apps:     make(map[string]apps.App),
		pulls:    make(map[string]bool),
		provider: provider,
	}
}

func (r *Reserved) Setup(conf config.Config) error {
	for name, repoConfig := range conf.Repository {
		parts := strings.Split(name, "/")
		org, repo := parts[0], parts[1]
		for _, branch := range repoConfig.AlwaysOn {
			app := apps.NewApp(org, repo, branch)
			r.Add(app, false)
		}
	}

	return nil
}

func (r *Reserved) Add(app apps.App, pull bool) {
	r.apps[app.String()] = app
	r.pulls[app.String()] = pull
	r.provider.Start(app)
}

func (r *Reserved) Remove(app apps.App) {
	delete(r.apps, app.String())
	r.provider.Stop(app)
}

func (r *Reserved) IsReserved(app apps.App) (bool, bool) {
	for _, a := range r.apps {
		if a.String() == app.String() {
			return true, r.pulls[a.String()]
		}
	}
	return false, false
}
