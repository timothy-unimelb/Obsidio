#!/bin/sh
set -eu

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  echo "usage: $0 <stage-name> <docker-build-context> [screen|full]" >&2
  exit 64
fi

stage_name=$1
build_context=$2
profile=${3:-full}
run_id=${RUN_ID:-}

case "$stage_name" in
  *[!a-z0-9-]*|'')
    echo "stage name must contain only lowercase letters, digits, and hyphens" >&2
    exit 64
    ;;
esac

case "$run_id" in
  *[!a-zA-Z0-9._-]*)
    echo "RUN_ID may contain only letters, digits, dots, underscores, and hyphens" >&2
    exit 64
    ;;
esac

case "$profile" in
  screen)
    workload_script="benchmarks/screening.js"
    profile_suffix="-screen"
    ;;
  full)
    workload_script="k6/grading.js"
    profile_suffix=""
    ;;
  *)
    echo "profile must be screen or full" >&2
    exit 64
    ;;
esac

if [ -z "$run_id" ]; then
  run_id="manual"
  result_suffix=""
else
  result_suffix="-${run_id}"
fi

image_name="obsidio-bench-${stage_name}:local"
container_name="obsidio-benchmark-${stage_name}-${profile}"
summary_path="benchmarks/results/${stage_name}${result_suffix}${profile_suffix}-summary.json"

if [ -e "$summary_path" ] && [ "${OVERWRITE_RESULTS:-0}" != "1" ]; then
  echo "result already exists: $summary_path" >&2
  echo "set a unique RUN_ID, or set OVERWRITE_RESULTS=1 only for disposable evidence" >&2
  exit 73
fi

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

mkdir -p benchmarks/results
docker build -t "$image_name" "$build_context"
if [ "${BENCH_CPU_MODE:-default}" = "portable" ]; then
  docker run -d --name "$container_name" --cpus=2 --memory=2g --memory-swap=2g -e GODEBUG=cpu.all=off -p 8080:8080 "$image_name" >/dev/null
else
  docker run -d --name "$container_name" --cpus=2 --memory=2g --memory-swap=2g -p 8080:8080 "$image_name" >/dev/null
fi

attempt=0
until curl -fsS http://127.0.0.1:8080/health >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "target did not become healthy" >&2
    exit 1
  fi
  sleep 1
done

set +e
k6 run --quiet --summary-export "$summary_path" -e TARGET=http://127.0.0.1:8080 "$workload_script"
k6_status=$?
set -e

if [ -f "$summary_path" ]; then
  node benchmarks/record-result.mjs "$stage_name" "$profile" "$run_id" "$summary_path" "$image_name" "$k6_status" "$workload_script"
fi

exit "$k6_status"
