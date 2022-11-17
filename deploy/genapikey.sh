#!/usr/bin/env bash
echo `LC_ALL=C tr -dc '[:alnum:]' < /dev/urandom | head -c64`
