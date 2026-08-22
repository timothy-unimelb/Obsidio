#!/bin/sh
# Persistence bonus check: write updates, hard-kill the container, restart it,
# and verify the updates survived. Usage: ./killtest.sh [image-tag]
set -eu
image=${1:-obsidio-go}
name=obsidio-killtest
docker rm -f "$name" >/dev/null 2>&1 || true
docker volume rm -f "$name-data" >/dev/null 2>&1 || true
docker build -q -t "$image" . >/dev/null
run() { docker run -d --name "$name" --cpus=2 --memory=2g -v "$name-data:/data" -p 8080:8080 "$image" >/dev/null; }
wait_healthy() { for _ in $(seq 1 30); do curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1 && return; sleep 1; done; echo "not healthy" >&2; exit 1; }
run; wait_healthy
curl -fsS -X POST -d '{"symbol":"AAPL","price":190}' http://127.0.0.1:8080/price >/dev/null
curl -fsS -X POST -d '{"symbol":"TSLA","price":251.5}' http://127.0.0.1:8080/price >/dev/null
docker kill "$name" >/dev/null
docker rm "$name" >/dev/null
run; wait_healthy
a=$(curl -fsS "http://127.0.0.1:8080/price?symbol=AAPL")
b=$(curl -fsS "http://127.0.0.1:8080/price?symbol=TSLA")
docker rm -f "$name" >/dev/null; docker volume rm "$name-data" >/dev/null
echo "$a"; echo "$b"
[ "$a" = '{"symbol":"AAPL","price":190}' ] && [ "$b" = '{"symbol":"TSLA","price":251.5}' ] && echo "PASS: updates survived docker kill" || { echo "FAIL" >&2; exit 1; }
