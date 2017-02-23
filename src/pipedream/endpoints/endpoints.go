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
	reserved    *Reserved
}

func NewHandler(conf config.Config, provider providers.Provider, g gh.GithubService) *gin.Engine {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*.tmpl")

	idle, _ := time.ParseDuration(conf.General.IdleShutdown.String())
	reserved := NewReserved(provider)
	if err := reserved.Setup(conf); err != nil {
		log.Fatal(err)
	}

	handler := Handler{
		provider: provider,
		lastRequest: LastRequest{
			idle:     idle,
			repos:    make(map[string]time.Time),
			reserved: reserved,
		},
		github:   g,
		reserved: reserved,
	}

	if err := handler.lastRequest.Setup(provider); err != nil {
		log.Fatal(err)
	}

	r.Use(handler.lastRequest.Middleware())

	r.POST("/hooks/:service", handler.getHook)
	r.GET("/app/:org/:repo/:branch", handler.appRequest)
	r.GET("/logs/:org/:repo/:branch", handler.getLogs)
	r.GET("/wait/:org/:repo/:branch", handler.wait)
	r.GET("/health/:org/:repo/:branch", handler.health)

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
		// TODO: put this logic into its own function or package
		parts := strings.Split(*event.Repo.FullName, "/")
		org, repo := parts[0], parts[1]
		parts = strings.Split(*event.Ref, "/")
		branch := parts[len(parts)-1]
		app := apps.NewApp(org, repo, branch)

		isReserved, isPull := h.reserved.IsReserved(app)
		if isReserved && isPull {
			// restart the app
			h.provider.Stop(app)
			h.provider.Start(app)
		}

	case *github.PullRequestEvent:
		// TODO: put this logic into its own function or package
		// detect app
		parts := strings.Split(*event.Repo.FullName, "/")
		org, repo := parts[0], parts[1]
		parts = strings.Split(*event.PullRequest.Head.Label, ":")
		branch := parts[1]
		app := apps.NewApp(org, repo, branch)

		if *event.PullRequest.State == "closed" {
			h.reserved.Remove(app)
		} else {
			h.reserved.Add(app, true)
		}
	}
	c.String(http.StatusOK, "OK")
}

func (h *Handler) getApp(c *gin.Context) apps.App {
	org := c.Param("org")
	repo := c.Param("repo")
	branch := c.Param("branch")
	return apps.NewApp(org, repo, branch)
}

func (h *Handler) health(c *gin.Context) {
	app := h.getApp(c)
	up := h.provider.IsAvailable(&url.URL{}, app)
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
	})
}

func (h *Handler) appRequest(c *gin.Context) {
	app := h.getApp(c)

	url := c.Request.URL
	ok := h.provider.IsAvailable(url, app)
	if ok {
		h.proxy(c, url)
		return
	} else {
		if err := h.provider.Start(app); err != nil {
			log.Printf("Couldn't start: %+v", err)
		}
		path := fmt.Sprintf("/wait/%s/%s/%s", app.Org, app.Repo, app.Branch)
		c.Redirect(http.StatusTemporaryRedirect, path)
	}
}

func (h *Handler) proxy(c *gin.Context, url *url.URL) {
	director := func(req *http.Request) {
		req.URL = url
	}
	proxy := &httputil.ReverseProxy{Director: director}
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
