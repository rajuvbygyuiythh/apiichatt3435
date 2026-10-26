#!/bin/sh
# Copies cmd/console/ui into desktop/frontend/dist (the Wails asset folder).
set -e
cd "$(dirname "$0")/.."
mkdir -p desktop/frontend/dist
rm -rf desktop/frontend/dist/*
cp -r cmd/console/ui/. desktop/frontend/dist/
echo "UI synced to desktop/frontend/dist"
