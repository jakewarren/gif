GIF_VERSION := $(shell git describe --tags HEAD)

all:
	go build -ldflags="-X github.com/jakewarren/gif/version.Version=$(GIF_VERSION)"

install: clean
	go install -ldflags="-X github.com/jakewarren/gif/version.Version=$(GIF_VERSION)"

clean:
	go clean -i
