package apps

import "fmt"

type App struct {
	Org    string `json:"org"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

func NewApp(org, repo, branch, commit string) App {
	if len(commit) >= 7 {
		commit = commit[0:7]
	}
	return App{
		Org:    org,
		Repo:   repo,
		Branch: branch,
		Commit: commit,
	}
}

func (a *App) String() string {
	return fmt.Sprintf("%s.%s.%s.%s", a.Org, a.Repo, a.Branch, a.Commit)
}
