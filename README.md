Pipedream
=========

Pipedream is an open source continuous integration and quality assurance tool
that runs per-commit copies of your application on demand or triggered by
Github webhooks.

Each time you open a Github pull request, a running instance of your
application is started for reviewers and QA members to test your changes on
a live instance.

### Features
 * Any version, on demand. Easily launch on demand a specific git commit just
   by going to the expected url (ie
`http://pipedream.chad.co/app/cbarraford/pipedream-simple/myCommit`)
 * Pull requests are king. When pull requests are opened/closed, Pipedream
   automatically starts/stops the instance for that branch. Making your change
readily available for your teammates to verify and test. When new commits are
added to the branch, the instance is restarted with the new changes
automatically.
 * Quick access via git commit status. Pipedream sends a Github commit status
   to each commit giving a quick link from your pull request to view that
version of your application.
 * Always available branches. Configure specific git branch to always be
   running an instance of your server (ie `master` or `staging`). Changes to
these branch will be automatically refreshed with the new commits.

### Configuration
Pipedream uses `gcfg` style configuration. Below is an example configuration
file.

```
[General]
# How long until an instance should be shut down due to inactivity
# Examples: 15s (15 seconds), 25m (25 minutes), 6h (6 hours)
IdleShutdown = 30m

# The base url to your server.
ServerAddress = "http://pipedream.chad.co"

# Docker configurations
# Docker API location
DockerHost = "tcp://localhost:2323" # defaults to "unix:///var/run/docker.sock"
# Host address to proxy requests to
DockerAddress = "localhost"

[Github]
# Github personal access token. Must have `repo:status`, `repo_deployment`,
`write:repo_hook`, `read:repo_hook`,
Token = XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
# A secret used to authenticate github web hooks. 
Secret = "shhh nomnomnom"

# for each supported repository, create one of these
# The name must be unique and follow "org/repo" naming convention.
[Repository "cbarraford/pipedream-simple"]
# if no branch is given, default to this branch
DefaultBranch = master
# list of branch to keep always keep running. You may specify this attribute
# more than once
AlwaysOn = master
AlwaysOn = staging
```

### Providers
Providers are backends to run your application on. Currently, only one
provider is support (docker), but more can be added later (ie digitial ocean,
ec2, Google App Engine, etc).

#### Docker Provider
You must supply Pipedream with a docker image to run your application on. For
a simple example, goto
[pipedream-simple](https://github.com/cbarraford/pipedream-simple).

When Pipedream starts a container, it will pass as an argument, the git commit
SHA on start. Your image is expected to utilize `ENTRYPOINT` to take that SHA
and start your application. Commonly, that may look like `git clone` your
repo, `git checkout` to the specific sha, do any building or compilation (ie
`npm install`, `bundle install`, `make XXXX`, etc) required and start the
service. Any application logs should output to stdout so they can be send to
`docker logs`.

### Development
Make sure you have `go` installed and `make`. To start the service... 
```
make build start
```
