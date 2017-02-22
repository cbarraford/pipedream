package apps

import "fmt"

type App struct {
	Org    string `json:"org"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

func NewApp(org, repo, branch string) App {
	return App{
		Org:    org,
		Repo:   repo,
		Branch: branch,
	}
}

func (a *App) String() string {
	return fmt.Sprintf("%s.%s.%s", a.Org, a.Repo, a.Branch)
}
