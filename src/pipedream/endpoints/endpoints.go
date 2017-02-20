package endpoints

import (
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/gin-gonic/gin"

	"pipedream/providers"
)

type Handler struct {
	provider    providers.Provider
	lastRequest LastRequest
}

func NewHandler(idle time.Duration, provider providers.Provider) *gin.Engine {
	r := gin.Default()

	handler := Handler{
		provider: provider,
		lastRequest: LastRequest{
			idle:  idle,
			repos: make(map[string]time.Time),
		},
	}

	r.Use(handler.lastRequest.Middleware())
	handler.lastRequest.StartTicker(provider)

	r.GET("/hook", handler.getHook)
	r.GET("/app/:org/:repo/:branch", handler.appRequest)
	r.GET("/logs/:org/:repo/:branch", handler.getLogs)

	return r
}

func (h *Handler) getHook(c *gin.Context) {
	c.String(http.StatusOK, "yes")
}

func (h *Handler) appRequest(c *gin.Context) {
	var err error
	org := c.Param("org")
	repo := c.Param("repo")
	branch := c.Param("branch")

	url := c.Request.URL
	ok := h.provider.IsAvailable(url, org, repo, branch)
	if ok {
		director := func(req *http.Request) {
			req.URL = url
		}
		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(c.Writer, c.Request)
		return
	}

	if err := h.provider.Start(org, repo, branch); err != nil {
		log.Printf("Couldn't start: %+v", err)
	}

	data, err := h.provider.GetLogs(org, repo, branch)
	if err != nil {
		log.Printf("Error getting logs: %+v", err)
	}

	// c.Header("Refresh", "5; url="+c.Request.URL.String())
	c.Data(200, "text/plain", data)
}

func (h *Handler) getLogs(c *gin.Context) {
	org := c.Param("org")
	repo := c.Param("repo")
	branch := c.Param("branch")

	data, err := h.provider.GetLogs(org, repo, branch)
	if err != nil {
		log.Printf("Error getting logs: %+v", err)
	}

	c.Data(200, "text/plain", data)
}
