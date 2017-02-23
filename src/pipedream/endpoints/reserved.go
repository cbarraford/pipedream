package endpoints

import (
	"strings"

	"pipedream/apps"
	"pipedream/config"
	"pipedream/providers"
)

type Reserved struct {
	apps     map[string]apps.App
	provider providers.Provider
}

func NewReserved(conf config.Config, provider providers.Provider) *Reserved {
	r := &Reserved{
		apps:     make(map[string]apps.App),
		provider: provider,
	}

	for name, repoConfig := range conf.Repository {
		parts := strings.Split(name, "/")
		org, repo := parts[0], parts[1]
		for _, branch := range repoConfig.AlwaysOn {
			app := apps.NewApp(org, repo, branch)
			r.Add(app)
		}
	}

	return r
}

func (r *Reserved) Add(app apps.App) {
	r.apps[app.String()] = app
	r.provider.Start(app)
}

func (r *Reserved) Remove(app apps.App) {
	delete(r.apps, app.String())
	r.provider.Stop(app)
}

func (r *Reserved) IsReserved(app apps.App) bool {
	for _, a := range r.apps {
		if a.String() == app.String() {
			return true
		}
	}
	return false
}
