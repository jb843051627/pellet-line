#!/usr/bin/env bash
set -euo pipefail
NAME="$1"
PLATFORM="$2"
IMAGE="benzhi/${NAME}:latest"
docker build --platform "$PLATFORM" -f benzhi.Dockerfile -t "$IMAGE" .
