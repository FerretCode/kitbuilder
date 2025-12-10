#!/bin/bash
set -e

GOOS=linux GOARCH=amd64 go build -o kitbuilder-linux-amd64
GOOS=windows GOARCH=amd64 go build -o kitbuilder-windows-amd64.exe
