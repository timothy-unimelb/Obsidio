#!/bin/sh
# Persistence-bonus check: POST two price updates through docker-compose.yml,
# hard-kill the container (docker kill, not a graceful stop), bring it back
# up, and verify both updates survived. Mirrors the organizer's stated
# verification: "design as if power is lost right after the 200 is sent."
#
# Usage: ./killtest.sh
set -eu
cd "$(dirname "$0")"

compose() { docker compose -f docker-compose.yml -p obsidio-killtest "$@"; }

cleanup() { compose down -v >/dev/null 2>&1 || true; }
trap cleanup EXIT

cleanup
compose build -q

wait_healthy() {
	for _ in $(seq 1 30); do
		curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1 && return 0
		sleep 1
	done
	echo "FAIL: container never became healthy" >&2
	exit 1
}

compose up -d
wait_healthy

curl -fsS -X POST -d '{"symbol":"AAPL","price":190}' http://127.0.0.1:8080/price >/dev/null
curl -fsS -X POST -d '{"symbol":"TSLA","price":251.5}' http://127.0.0.1:8080/price >/dev/null

cid=$(compose ps -q app)
docker kill "$cid" >/dev/null

compose up -d
wait_healthy

a=$(curl -fsS "http://127.0.0.1:8080/price?symbol=AAPL")
b=$(curl -fsS "http://127.0.0.1:8080/price?symbol=TSLA")

echo "$a"
echo "$b"

if [ "$a" = '{"symbol":"AAPL","price":190}' ] && [ "$b" = '{"symbol":"TSLA","price":251.5}' ]; then
	echo "PASS: updates survived docker kill"
else
	echo "FAIL: updates did not survive docker kill" >&2
	exit 1
fi
