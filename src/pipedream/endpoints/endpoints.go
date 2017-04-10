package endpoints

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/github"

	"pipedream/apps"
	"pipedream/config"
	"pipedream/providers"
	gh "pipedream/services/github"
)

type Handler struct {
	provider    providers.Provider
	lastRequest LastRequest
	github      gh.GithubService
}

func NewHandler(conf config.Config, provider providers.Provider, g gh.GithubService) *gin.Engine {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*.tmpl")

	idle, _ := time.ParseDuration(conf.General.IdleShutdown.String())

	handler := Handler{
		lastRequest: LastRequest{
			idle:     idle,
			repos:    make(map[string]time.Time),
			pulls:    make(map[string]bool),
			alwaysOn: make(map[string][]string),
			github:   g,
			config:   conf,
		},
		provider: provider,
		github:   g,
	}

	if err := handler.lastRequest.Setup(provider); err != nil {
		log.Fatal(err)
	}

	r.Use(handler.lastRequest.Middleware())

	r.POST("/hooks/:service", handler.getHook)

	r.Any("/app/:org/:repo/:commit", handler.commitRequest)
	r.Any("/app/:org/:repo/:commit/*path", handler.commitRequest)

	r.GET("/logs/:org/:repo/:commit", handler.getLogs)
	r.GET("/wait/:org/:repo/:commit", handler.wait)
	r.GET("/health/:org/:repo/:commit", handler.health)

	return r
}

func (h *Handler) getHook(c *gin.Context) {
	payload, err := github.ValidatePayload(c.Request, []byte(h.github.Secret))
	if err != nil {
		c.Error(err)
	}

	hookType := github.WebHookType(c.Request)
	event, err := github.ParseWebHook(hookType, payload)
	if err != nil {
		c.Error(err)
	}

	switch event := event.(type) {
	case *github.PingEvent:
		// do nothing
	case *github.PushEvent:
		parts := strings.Split(*event.Repo.FullName, "/")
		org, repo := parts[0], parts[1]
		parts = strings.Split(*event.Ref, "/")
		branch := parts[len(parts)-1]
		commit := *event.Commits[len(*event.Commits)-1].ID
		app := apps.NewApp(org, repo, branch, commit)

		serverAddress := h.lastRequest.config.General.ServerAddress
		url := fmt.Sprintf("%s/app/%s/%s/%s", serverAddress, app.Org, app.Repo, app.Commit)
		if err := h.github.CreateStatus(url, app.Org, app.Repo, *event.Commits[0].ID, "success"); err != nil {
			log.Printf("%+v", err)
		}

	case *github.PullRequestEvent:
		// detect app
		parts := strings.Split(*event.Repo.FullName, "/")
		org, repo := parts[0], parts[1]
		parts = strings.Split(*event.PullRequest.Head.Label, ":")
		branch := parts[1]
		commit := *event.PullRequest.Head.SHA
		app := apps.NewApp(org, repo, branch, commit)

		if *event.PullRequest.State == "closed" {
			h.provider.Stop(app)
		} else {
			h.provider.Start(app)
		}
	}
	c.String(http.StatusOK, "OK")
}

func (h *Handler) getApp(c *gin.Context) apps.App {
	org := c.Param("org")
	repo := c.Param("repo")
	branch := c.Param("branch")
	commit := c.Param("commit")
	return apps.NewApp(org, repo, branch, commit)
}

func (h *Handler) health(c *gin.Context) {
	app := h.getApp(c)
	state := h.provider.State(app)
	up := false
	if state == providers.AppUp {
		up = true
	}
	requestTime := h.lastRequest.Get(app)

	c.JSON(200, gin.H{
		"app":          app,
		"up":           up,
		"last_request": requestTime,
	})
}

func (h *Handler) wait(c *gin.Context) {
	app := h.getApp(c)
	c.HTML(http.StatusOK, "wait.tmpl", gin.H{
		"org":    app.Org,
		"repo":   app.Repo,
		"branch": app.Branch,
		"commit": app.Commit,
	})
}

func (h *Handler) commitRequest(c *gin.Context) {
	app := h.getApp(c)
	path := c.Param("path")

	originalRequestHeader := h.lastRequest.config.General.OrginalRequestHeader
	if originalRequestHeader != "" {
		url := url.URL{
			Scheme: "http",
			Host:   c.Request.Host,
			Path:   c.Request.URL.Path,
		}
		c.Request.Header.Set(originalRequestHeader, url.String())
	}

	state := h.provider.State(app)
	ok := h.provider.ModifyURL(c.Request, app)
	c.Request.URL.Path = path
	if ok && state == providers.AppUp {
		h.proxy(c, c.Request.URL)
		return
	} else {
		if err := h.provider.Start(app); err != nil {
			log.Printf("Couldn't start: %+v", err)
		}
		h.lastRequest.AddRequest(app)
		waitPath := fmt.Sprintf("/wait/%s/%s/%s", app.Org, app.Repo, app.Commit)
		c.Redirect(http.StatusTemporaryRedirect, waitPath)
	}
}

func (h *Handler) proxy(c *gin.Context, url *url.URL) {
	app := h.getApp(c)
	director := func(req *http.Request) {
		req.URL = url
	}
	modResponse := func(w *http.Response) error {
		if w.StatusCode >= 300 && w.StatusCode < 400 {
			location := w.Header.Get("Location")
			// if location is a fully qualified url, don't alter it
			if strings.HasPrefix(location, "http") {
				return nil
			}
			path := fmt.Sprintf("/app/%s/%s/%s%s", app.Org, app.Repo, app.Commit, location)
			c.Redirect(w.StatusCode, path)
		}
		return nil
	}
	proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: modResponse}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (h *Handler) getLogs(c *gin.Context) {
	app := h.getApp(c)

	pipeReader, pipeWriter := io.Pipe()
	defer pipeWriter.Close()
	defer pipeReader.Close()
	err := h.provider.GetLogs(pipeWriter, app)
	if err != nil {
		log.Printf("Error getting logs: %+v", err)
	}

	c.Stream(func(w io.Writer) bool {
		scanner := bufio.NewScanner(pipeReader)
		for scanner.Scan() {
			c.SSEvent("log", scanner.Text())
			c.Writer.Flush()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "There was an error with the scanner in attached container", err)
			return false
		}
		return true
	})
}
