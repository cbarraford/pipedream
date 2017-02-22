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
	"time"

	"github.com/gin-gonic/gin"

	"pipedream/apps"
	"pipedream/config"
	"pipedream/providers"
)

type Handler struct {
	provider    providers.Provider
	lastRequest LastRequest
}

func NewHandler(conf config.Config, provider providers.Provider) *gin.Engine {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*.tmpl")

	idle, _ := time.ParseDuration(conf.General.IdleShutdown.String())

	handler := Handler{
		provider: provider,
		lastRequest: LastRequest{
			idle:  idle,
			repos: make(map[string]time.Time),
			conf:  conf,
		},
	}

	// populate last request
	applications, err := provider.ListApps()
	if err != nil {
		log.Fatal(err)
	}
	for _, app := range applications {
		handler.lastRequest.AddRequest(app)
	}

	r.Use(handler.lastRequest.Middleware())
	handler.lastRequest.StartTicker(provider)

	r.GET("/hook", handler.getHook)
	r.GET("/app/:org/:repo/:branch", handler.appRequest)
	r.GET("/logs/:org/:repo/:branch", handler.getLogs)
	r.GET("/wait/:org/:repo/:branch", handler.wait)
	r.GET("/health/:org/:repo/:branch", handler.health)

	return r
}

func (h *Handler) getHook(c *gin.Context) {
	c.String(http.StatusOK, "yes")
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
	err := h.provider.GetLogs(pipeWriter, app)
	if err != nil {
		log.Printf("Error getting logs: %+v", err)
	}

	func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			c.Writer.WriteString(fmt.Sprintf("%s\n", scanner.Text()))
			c.Writer.Flush()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "There was an error with the scanner in attached container", err)
		}
	}(pipeReader)
}
