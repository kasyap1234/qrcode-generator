#!/bin/bash

set -e
echo "STarting redis container"
docker run -d --name redis -p 6379:6379 redis:latest
echo "Building go app"
go build -o qrgen main.go
echo "Running go app"

./qrgen
