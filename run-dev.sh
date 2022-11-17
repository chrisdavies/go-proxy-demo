#!/usr/bin/env bash
systemd-socket-activate --listen=8000 --listen=8001 \
  -E API_KEY=topsecret \
  -E V1_HOST=localhost:3000 \
  -E V2_HOST=localhost:4000 \
  -E IGNORE_TLS=true \
  -E DEV=true \
  -E CGO_ENABLED=1 \
  reflex -r '\.go' -s -- sh -c 'go run ./src/'

