package providers

import (
	"net/url"

	"pipedream/apps"
)

// Provider is the backend that application run within. We use an interface so
// we can support multiple potential backends (ie docker, digitalocean, etc)
type Provider interface {
	// Name of provider
	Name() string

	// Start app
	Start(app apps.App) error

	// Stop app
	Stop(app apps.App) error

	// Is app available for traffic
	// If app is available, url should be updated to proxy location
	IsAvailable(url *url.URL, app apps.App) bool

	// Get application logs
	GetLogs(app apps.App) ([]byte, error)

	// List all apps
	ListApps() ([]apps.App, error)
}
