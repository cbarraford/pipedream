package main

import (
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/gin-gonic/gin"

	"pipedream/providers/docker"
)

func main() {
	r := gin.Default()

	// TODO: make this configurable (pick own provider)
	provider, err := docker.NewProvider()
	if err != nil {
		panic(err)
	}

	r.GET("/hook", func(c *gin.Context) {
		c.String(200, "yes")
	})

	r.GET("/app/:org/:repo/:branch", func(c *gin.Context) {
		var err error
		org := c.Param("org")
		repo := c.Param("repo")
		branch := c.Param("branch")

		url := c.Request.URL
		ok := provider.IsAvailable(url, org, repo, branch)
		if ok {
			director := func(req *http.Request) {
				req.URL = url
			}
			proxy := &httputil.ReverseProxy{Director: director}
			proxy.ServeHTTP(c.Writer, c.Request)
			return
		}

		if err := provider.Start(org, repo, branch); err != nil {
			log.Printf("Couldn't start: %+v", err)
		}

		data, err := provider.GetLogs(org, repo, branch)
		if err != nil {
			log.Printf("Error getting logs: %+v", err)
		}

		// c.Header("Refresh", "5; url="+c.Request.URL.String())
		c.Data(200, "text/plain", data)
	})

	r.GET("/logs/:org/:repo/:branch", func(c *gin.Context) {
		org := c.Param("org")
		repo := c.Param("repo")
		branch := c.Param("branch")

		data, err := provider.GetLogs(org, repo, branch)
		if err != nil {
			log.Printf("Error getting logs: %+v", err)
		}

		c.Data(200, "text/plain", data)
	})

	r.Run() // listen and serve on 0.0.0.0:8080
}
