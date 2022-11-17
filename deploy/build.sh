#!/usr/bin/env bash

# Ensure errors are handled properly, debugging is easier, etc.
set -o errexit
set -o nounset
set -o pipefail

# Run the rest of the script from the script's directory.
cd "$(dirname "$0")"

# Change to the source directory
cd ../src/

# Ensure the destination directory exists
mkdir -p ../dist/

# Clear previous build artifacts
if [[ "$(ls ../dist/)" ]]; then
  rm ../dist/go-proxy-demo*
fi

# Create the new build artifacts
filename="go-proxy-demo-`date "+%Y%m%d%H%M%S"`"

CGO_ENABLED=1 go build --tags "linux" -o "../dist/$filename" .
ln -s "./$filename" ../dist/go-proxy-demo

