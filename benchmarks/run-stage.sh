#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <stage-name> <docker-build-context>" >&2
  exit 64
fi

stage_name=$1
build_context=$2
image_name="obsidio-bench-${stage_name}:local"
container_name="obsidio-benchmark-${stage_name}"
summary_path="benchmarks/results/${stage_name}-summary.json"

case "$stage_name" in
  *[!a-z0-9-]*|'')
    echo "stage name must contain only lowercase letters, digits, and hyphens" >&2
    exit 64
    ;;
esac

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

mkdir -p benchmarks/results
docker build -t "$image_name" "$build_context"
docker run -d --name "$container_name" --cpus=2 --memory=2g -p 8080:8080 "$image_name" >/dev/null

attempt=0
until curl -fsS http://127.0.0.1:8080/health >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "target did not become healthy" >&2
    exit 1
  fi
  sleep 1
done

k6 run --summary-export "$summary_path" -e TARGET=http://127.0.0.1:8080 k6/grading.js
