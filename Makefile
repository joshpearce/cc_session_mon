.PHONY: all deps build run test lint clean

# Project name
NAME := cc_session_mon

all: deps build

deps:
	go mod tidy

build:
	go build -o bin/$(NAME) .

run:
	go run .

test:
	go test -v ./...

lint:
	go tool golangci-lint run

clean:
	rm -rf bin/
