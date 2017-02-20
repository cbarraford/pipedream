package providers

import "net/url"

// Provider is the backend that application run within. We use an interface so
// we can support multiple potential backends (ie docker, digitalocean, etc)
type Provider interface {
	// Name of provider
	Name() string

	// Start app
	Start(org, repo, branch string) error

	// Stop app
	Stop(org, repo, branch string) error

	// Is app available for traffic
	// If app is available, url should be updated to proxy location
	IsAvailable(url *url.URL, org, repo, branch string) bool

	// Get application logs
	GetLogs(org, repo, branch string) ([]byte, error)
}
