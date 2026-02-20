.PHONY: help all deps build run test lint clean

# Project name
NAME := cc_session_mon

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  deps   - Run go mod tidy"
	@echo "  build  - Build binary to bin/$(NAME)"
	@echo "  run    - Run the application"
	@echo "  test   - Run tests"
	@echo "  lint   - Run golangci-lint"
	@echo "  clean  - Remove build artifacts"

all: deps build

deps:
	go mod download
	go mod verify

build:
	go build -o bin/$(NAME) .

run:
	go run .

test:
	go test -v ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
