package docker

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"pipedream/config"

	docker "github.com/fsouza/go-dockerclient"
)

type Docker struct {
	client *docker.Client
	conf   config.Config
}

func NewProvider(conf config.Config) (Docker, error) {
	// Try to load client from env first, then try unix socket
	client, err := docker.NewClientFromEnv()
	if err != nil {
		// TODO: make endpoint configurable
		endpoint := "unix:///var/run/docker.sock"
		client, err = docker.NewClient(endpoint)
	}
	return Docker{
		client: client,
		conf:   conf,
	}, err
}

func (p Docker) Name() string {
	return "docker"
}

func (p Docker) Start(org, repo, branch string) error {
	var err error
	container, ok := p.getContainer(org, repo, branch)
	if !ok {
		container, err = p.createContainer(org, repo, branch)
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

func (p Docker) Stop(org, repo, branch string) error {
	return p.removeContainer(org, repo, branch)
}

func (p Docker) IsAvailable(url *url.URL, org, repo, branch string) bool {
	// does container exist?
	container, ok := p.getContainer(org, repo, branch)
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
		url.Scheme = "http"
		// TODO: make this configurable
		url.Host = fmt.Sprintf("localhost:%s", port)
		return true
	}

	return false
}

func (p Docker) GetLogs(org, repo, branch string) ([]byte, error) {
	return p.getLogs(org, repo, branch)
}

func (p Docker) containerName(org, repo, branch string) string {
	return fmt.Sprintf("%s-%s-%s", org, repo, branch)
}

func (p Docker) createContainer(org, repo, branch string) (*docker.Container, error) {
	// get repo configuration (if exists)
	repoConf, _ := p.conf.GetRepo(org, repo)

	container_id := p.containerName(org, repo, branch)
	containerConfig := docker.Config{
		AttachStdout: true,
		AttachStdin:  true,
		Image:        "simple", // TODO: make this configurable
		Hostname:     container_id,
		Cmd:          []string{branch},
	}

	// default restart policy
	restart := docker.NeverRestart()
	// if this branch is AlwaysOn, set policy accordingly
	for _, rname := range repoConf.AlwaysOn {
		if branch == rname {
			restart = docker.RestartOnFailure(10)
			break
		}
	}

	contHostConfig := docker.HostConfig{
		PublishAllPorts: true,
		Privileged:      true,
		RestartPolicy:   restart,
	}
	opts := docker.CreateContainerOptions{Name: container_id, Config: &containerConfig, HostConfig: &contHostConfig}
	return p.client.CreateContainer(opts)
}

func (p Docker) startContainer(container *docker.Container) error {
	return p.client.StartContainer(container.ID, container.HostConfig)
}

func (p Docker) removeContainer(org, repo, branch string) error {
	container_id := p.containerName(org, repo, branch)
	config := docker.RemoveContainerOptions{
		ID:    container_id,
		Force: true,
	}
	return p.client.RemoveContainer(config)
}

func (p Docker) getContainer(org, repo, branch string) (*docker.Container, bool) {
	container_id := p.containerName(org, repo, branch)
	container, err := p.client.InspectContainer(container_id)
	if err != nil {
		log.Printf("Could not get container: %+v", err)
		return container, false
	}
	return container, true
}

func (p Docker) getLogs(org, repo, branch string) ([]byte, error) {
	var err error
	stderrBuffer := new(bytes.Buffer)
	err = p.client.Logs(docker.LogsOptions{
		Container:   p.containerName(org, repo, branch),
		ErrorStream: stderrBuffer,
		Stdout:      true,
		Stderr:      true,
		Tail:        "20",
	})
	if err != nil {
		log.Printf("Error getting logs: %+v", err)
		return nil, err
	}
	return stderrBuffer.Bytes(), nil
}
