#!/usr/bin/env bash

set -e

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

rm -rf "$ROOT_DIR/dist/web"
mkdir -p "$ROOT_DIR/dist"

#cd "$ROOT_DIR/backend" && CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -linkmode external -extldflags -static" -o "$ROOT_DIR/dist/lazyiso"

pushd "$ROOT_DIR/ui" >/dev/null
npx vite build --outDir "$ROOT_DIR/dist/web"
popd >/dev/null

mkdir -p "$ROOT_DIR/dist/normalization/rules"
cp -f "$ROOT_DIR"/backend/normalization/rules/*.json "$ROOT_DIR/dist/normalization/rules/"
