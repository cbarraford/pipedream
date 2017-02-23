package apps

import (
	b64 "encoding/base64"
	"fmt"
)

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

func (a *App) Hash() string {
	return b64.StdEncoding.EncodeToString([]byte(a.String()))
}
