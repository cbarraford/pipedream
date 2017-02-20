PWD=$(shell pwd)
export GOPATH=${PWD}/_vendor:${PWD}

build:
	go get -t -v ./src/pipedream/...

start:
	./bin/pipedream
