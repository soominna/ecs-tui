VERSION ?= dev

.PHONY: build release-dry

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o ecs-tui .

release-dry:
	goreleaser release --snapshot --clean
