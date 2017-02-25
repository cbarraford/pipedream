package providers

import (
	"io"
	"net/http"

	"pipedream/apps"
)

type State int

const (
	AppDown State = iota
	AppUp   State = iota
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

	// Current state of app
	State(app apps.App) State

	// Get url to application
	ModifyURL(req *http.Request, app apps.App) bool

	// Get application logs
	GetLogs(w *io.PipeWriter, app apps.App) error

	// List all apps
	ListApps() ([]apps.App, error)
}
