#!/bin/bash
set -e

go vet ./...
go tool govulncheck ./...
go tool golangci-lint run ./...