#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
GOOS="${GOOS:-$(go env GOOS)}" GOARCH="${GOARCH:-$(go env GOARCH)}" GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o dist/probelune-node ./cmd/node
