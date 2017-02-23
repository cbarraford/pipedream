package github

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var ctx = context.Background()
var noHook = github.Hook{}

type GithubService struct {
	Client        *github.Client
	ServerAddress string
	Secret        string
}

func NewClient(token, serverAddress, secret string) GithubService {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return GithubService{
		Client:        github.NewClient(tc),
		ServerAddress: serverAddress,
		Secret:        secret,
	}
}

func (g *GithubService) Name() string {
	return "pipedream"
}

func (g *GithubService) GetReference(ref string) (string, error) {
	ref = fmt.Sprintf("heads/%s", ref)
	reference, _, err := g.Client.Git.GetRef(ctx, "cbarraford", "pipedream-simple", ref)
	if err != nil {
		return "", err
	}
	return *reference.Object.SHA, err
}

func (g *GithubService) CreateStatus(ref string, state string) error {
	description := "Pipedream Instance"
	url := fmt.Sprintf("http://localhost:8080/appByCommit/cbarraford/pipedream-simple/%s", ref)
	repoStatus := &github.RepoStatus{
		State:       &state,
		TargetURL:   &url,
		Description: &description,
	}

	_, _, err := g.Client.Repositories.CreateStatus(ctx, "cbarraford", "pipedream-simple", ref, repoStatus)
	return err
}

func (g *GithubService) ListOpenPullRequests(org, repo string) ([]*github.PullRequest, error) {
	opts := github.PullRequestListOptions{}
	pulls, _, err := g.Client.PullRequests.List(ctx, org, repo, &opts)
	if err != nil {
		return nil, err
	}
	return pulls, err
}

func (g *GithubService) ProperHook() *github.Hook {
	name := "web"
	url := fmt.Sprintf("%s/hooks/github", g.ServerAddress)
	active := true
	config := make(map[string]interface{})
	config["url"] = url
	config["secret"] = g.Secret
	config["content_type"] = "json"
	return &github.Hook{
		Name:   &name,
		URL:    &url,
		Active: &active,
		Events: []string{"push", "pull_request"},
		Config: config,
	}
}

func (g *GithubService) Setup() error {
	hook, err := g.GetHook()
	if err == nil {
		// hook exists, update it
		_ = hook
	} else {
		// hook DOES NOT exists, create it
		return g.CreateHook()
	}
	return nil
}

func (g *GithubService) GetHook() (*github.Hook, error) {
	listOptions := github.ListOptions{}
	hooks, _, err := g.Client.Repositories.ListHooks(
		ctx, "cbarraford", "pipedream-simple", &listOptions,
	)
	if err != nil {
		return &noHook, err
	}

	properHook := g.ProperHook()
	for _, hook := range hooks {
		log.Printf("Hook: %+v", hook)
		if hook.Config["url"] == properHook.Config["url"] {
			return hook, nil
		}
	}
	return &noHook, errors.New("Hook not found")
}

func (g *GithubService) CreateHook() error {
	_, _, err := g.Client.Repositories.CreateHook(
		ctx,
		"cbarraford",
		"pipedream-simple",
		g.ProperHook(),
	)
	return err
}
