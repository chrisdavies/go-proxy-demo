#!/usr/bin/env bash

# Ensure errors are handled properly, debugging is easier, etc.
set -o errexit
set -o nounset
set -o pipefail

printUsage() {
    echo '
Usage: ./deploy.sh {hostname-or-ip}

Build and deploy the proxy service.
'
    exit 1
}

main() {
  # Process commandline flags
  if [[ $# != 1 ]]; then
    printUsage
  fi

  # The host where we're going to ship our build artifacts
  hostname=$1

  # Run the rest of the script from the script's directory.
  cd "$(dirname "$0")"

  # Just in case, rebuild...
  echo "Building..."
  ./build.sh

  # Change to the folder where the build artifacts were produced
  cd ../dist/

  # Rsync the build artifacts
  echo "Copying to $hostname..."
  rsync --links -r -e ssh . "example@$hostname:/home/example/go-proxy-demo/"

  # Restart the remote service
  echo "Restarting service on $hostname..."
  ssh "example@$hostname" 'sudo systemctl restart go-proxy-demo.service'

  # Removing old artifacts
  # ls -t                 sorts newest to oldest
  # grep go-proxy-demo-20 matches our timestamped binaries
  # sed -e '1,10d'        removes the first 10 (most recent 10) names
  # xargs -r -d '\n' rm   runs rm on the remaining file names, if any
  echo "Removing old artifacts on $hostname..."
  ssh example@$hostname "cd /home/example/go-proxy-demo/ && ls -t | grep go-proxy-demo-20 | sed -e '1,10d' | xargs -r -d '\n' rm"
}

main "$@"

