#!/usr/bin/env bash
# Cross-compiles release binaries for each supported platform, named
# dir-age_<version>_<os>_<arch>[.exe]. Invoked by semantic-release
# (see .releaserc.json) with the version it decided on as the only argument.
set -euo pipefail

version="${1:?usage: build-release-assets.sh <version>}"

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
)

rm -rf dist
mkdir -p dist

for target in "${targets[@]}"; do
  read -r goos goarch <<<"$target"
  ext=""
  [ "$goos" = "windows" ] && ext=".exe"
  out="dist/dir-age_${version}_${goos}_${goarch}${ext}"
  echo "building ${out}"
  GOOS="$goos" GOARCH="$goarch" go build -ldflags "-s -w" -o "$out" .
done
