#!/usr/bin/env bash

set -e

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

rm -rf "$ROOT_DIR/dist/web"
mkdir -p "$ROOT_DIR/dist"

pushd "$ROOT_DIR/backend" >/dev/null
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o "$ROOT_DIR/dist/lazymanga" .
popd >/dev/null

pushd "$ROOT_DIR/ui" >/dev/null
npm ci --no-audit --no-fund
npx vite build --outDir "$ROOT_DIR/dist/web"
popd >/dev/null

mkdir -p "$ROOT_DIR/dist/normalization/rules"
cp -f "$ROOT_DIR"/backend/normalization/rules/*.json "$ROOT_DIR/dist/normalization/rules/"
