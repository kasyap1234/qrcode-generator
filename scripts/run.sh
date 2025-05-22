#!/bin/bash
set -e
echo " Building go app"
go build -o qrgen main.go
echo "Running go app"
./qrgen
