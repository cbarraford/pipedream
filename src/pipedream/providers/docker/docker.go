package docker

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"pipedream/apps"
	"pipedream/config"
	"pipedream/providers"
)

type Docker struct {
	client *docker.Client
	conf   config.Config
}

func NewProvider(conf config.Config) (Docker, error) {
	endpoint := conf.General.DockerHost
	if endpoint == "" {
		endpoint = "unix:///var/run/docker.sock"
	}
	client, err := docker.NewClient(endpoint)
	return Docker{
		client: client,
		conf:   conf,
	}, err
}

func (p Docker) Name() string {
	return "docker"
}

func (p Docker) Start(app apps.App) error {
	var err error
	container, ok := p.getContainer(app)
	if !ok {
		container, err = p.createContainer(app)
		if err != nil {
			return err
		}
		err = p.startContainer(container)
	} else {
		if !container.State.Running {
			err = p.startContainer(container)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p Docker) Stop(app apps.App) error {
	return p.removeContainer(app)
}

func (p Docker) State(app apps.App) providers.State {
	req, _ := http.NewRequest("GET", "", nil)

	ok := p.ModifyURL(req, app)
	if !ok {
		return providers.AppDown
	}

	repoConf, _ := p.conf.GetRepo(app.Org, app.Repo)

	req.URL.Path = repoConf.HealthCheckPath
	response, err := http.Get(req.URL.String())
	if err != nil {
		return providers.AppDown
	}

	// check that we have a 200 response
	if response.StatusCode >= 300 {
		return providers.AppDown
	}

	return providers.AppUp
}

func (p Docker) ModifyURL(req *http.Request, app apps.App) bool {
	container, ok := p.getContainer(app)
	if !ok {
		return false
	}

	// is container running?
	if !container.State.Running {
		return false
	}

	// is network port available?
	if container.NetworkSettings == nil {
		return false
	}

	for _, k := range container.NetworkSettings.Ports {
		port := k[0].HostPort
		host := p.conf.General.DockerAddress
		if host == "" {
			host = "localhost"
		}

		req.URL.Scheme = "http"
		req.URL.Host = fmt.Sprintf("%s:%s", host, port)
		return true
	}

	return false
}

func (p Docker) GetLogs(w *io.PipeWriter, app apps.App) error {
	return p.getLogs(w, app)
}

func (p Docker) ListApps() ([]apps.App, error) {
	return p.listApps()
}

func (p Docker) listApps() (applications []apps.App, err error) {
	containerOptions := docker.ListContainersOptions{}
	containers, err := p.client.ListContainers(containerOptions)
	if err != nil {
		return
	}

	for _, container := range containers {
		name := strings.Trim(container.Names[0], "/")
		parts := strings.Split(name, ".")
		if len(parts) > 2 {
			app := apps.NewApp(parts[0], parts[1], "", parts[2])
			applications = append(applications, app)
		}
	}

	return
}

func (p Docker) createContainer(app apps.App) (*docker.Container, error) {
	// get repo configuration (if exists)
	repoConf, _ := p.conf.GetRepo(app.Org, app.Repo)

	containerConfig := docker.Config{
		AttachStdout: true,
		AttachStdin:  true,
		Image:        repoConf.DockerImage,
		Hostname:     p.containerName(app),
		Cmd:          []string{app.Commit},
	}

	// default restart policy
	restart := docker.NeverRestart()
	// if this branch is AlwaysOn, set policy accordingly
	for _, rname := range repoConf.AlwaysOn {
		if app.Branch == rname {
			restart = docker.RestartOnFailure(10)
			break
		}
	}

	contHostConfig := docker.HostConfig{
		PublishAllPorts: true,
		Privileged:      true,
		RestartPolicy:   restart,
	}
	opts := docker.CreateContainerOptions{Name: p.containerName(app), Config: &containerConfig, HostConfig: &contHostConfig}
	return p.client.CreateContainer(opts)
}

func (p Docker) startContainer(container *docker.Container) error {
	return p.client.StartContainer(container.ID, container.HostConfig)
}

func (p Docker) removeContainer(app apps.App) error {
	config := docker.RemoveContainerOptions{
		ID:    p.containerName(app),
		Force: true,
	}
	return p.client.RemoveContainer(config)
}

func (p Docker) getContainer(app apps.App) (*docker.Container, bool) {
	container, err := p.client.InspectContainer(p.containerName(app))
	if err != nil {
		log.Printf("Could not get container: %+v", err)
		return container, false
	}
	return container, true
}

func (p Docker) getLogs(w *io.PipeWriter, app apps.App) error {
	go p.client.Logs(docker.LogsOptions{
		Container:    p.containerName(app),
		OutputStream: w,
		ErrorStream:  w,
		Stdout:       true,
		Stderr:       true,
		//Tail:         "20",
		Follow:      true,
		RawTerminal: true,
		//Timestamps:   true,
	})
	return nil
}

func (p Docker) containerName(app apps.App) string {
	return fmt.Sprintf("%s.%s.%s", app.Org, app.Repo, app.Commit)
}
