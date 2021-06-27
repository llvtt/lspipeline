#!/bin/sh -e

go build -o lspipeline cmd/lspipeline/main.go

if [ "$1" = "deploy" ]; then
  cp ./lspipeline ~/local/bin/lspipeline
fi
