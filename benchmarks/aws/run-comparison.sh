#!/usr/bin/env bash

set -euo pipefail

AWS_BENCH_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$AWS_BENCH_DIR/../.." && pwd)
source "$AWS_BENCH_DIR/lib.sh"

champion_context=${1:-}
candidate_context=${2:-}
profile=${3:-screen}
comparison_set=${BENCH_COMPARISON_SET:-}
run_prefix=${RUN_PREFIX:-}
sequence=${BENCH_SEQUENCE:-"A B A"}

if [[ -z "$champion_context" || -z "$candidate_context" || -z "$comparison_set" || -z "$run_prefix" ]]; then
  echo "usage: BENCH_COMPARISON_SET=<set> RUN_PREFIX=<prefix> $0 <champion-context> <candidate-context> [screen|full]" >&2
  exit 64
fi

case "$profile" in
  screen) workload="$REPO_ROOT/benchmarks/screening.js" ;;
  full) workload="$REPO_ROOT/k6/grading.js" ;;
  *) echo "profile must be screen or full" >&2; exit 64 ;;
esac

safe_set=$(printf '%s' "$comparison_set" | tr -cd 'a-zA-Z0-9_.-')
if [[ -z "$safe_set" || "$safe_set" != "$comparison_set" ]]; then
  echo "comparison set may contain only letters, digits, dot, underscore, and hyphen" >&2
  exit 64
fi

for context in "$champion_context" "$candidate_context"; do
  if [[ ! -f "$context/Dockerfile" ]]; then
    echo "Docker build context is missing Dockerfile: $context" >&2
    exit 66
  fi
  repo=$(git -C "$context" rev-parse --show-toplevel)
  if [[ -n $(git -C "$repo" status --porcelain -- "$context") ]]; then
    echo "Remote comparisons require committed source: $context" >&2
    exit 65
  fi
done

require_aws_identity
require_command node
require_command scp
require_command ssh
require_command tar

target_public=$(stack_output TargetPublicIp)
target_private=$(stack_output TargetPrivateIp)
load_public=$(stack_output LoadPublicIp)
environment_id=$(stack_output EnvironmentId)
target_instance_id=$(stack_output TargetInstanceId)
load_instance_id=$(stack_output LoadInstanceId)
champion_sha=$(git -C "$champion_context" rev-parse HEAD)
candidate_sha=$(git -C "$candidate_context" rev-parse HEAD)

ssh_args=(
  -i "$AWS_BENCH_KEY_PATH"
  -o "UserKnownHostsFile=$AWS_BENCH_KNOWN_HOSTS"
  -o StrictHostKeyChecking=accept-new
  -o ConnectTimeout=10
)
scp_args=(
  -i "$AWS_BENCH_KEY_PATH"
  -o "UserKnownHostsFile=$AWS_BENCH_KNOWN_HOSTS"
  -o StrictHostKeyChecking=accept-new
  -o ConnectTimeout=10
)

temporary_dir=$(mktemp -d)
cleanup() {
  rm -rf "$temporary_dir"
  ssh "${ssh_args[@]}" "ec2-user@$target_public" 'docker rm -f obsidio-remote-benchmark >/dev/null 2>&1 || true' >/dev/null 2>&1 || true
}
trap cleanup EXIT

tar -C "$champion_context" -czf "$temporary_dir/champion.tgz" .
tar -C "$candidate_context" -czf "$temporary_dir/candidate.tgz" .
cp "$workload" "$temporary_dir/workload.js"

remote_root="/opt/obsidio/$safe_set"
ssh "${ssh_args[@]}" "ec2-user@$target_public" "mkdir -p '$remote_root/champion' '$remote_root/candidate'"
ssh "${ssh_args[@]}" "ec2-user@$load_public" "mkdir -p '$remote_root/results' '$remote_root/workload' && chmod 777 '$remote_root/results'"
scp "${scp_args[@]}" "$temporary_dir/champion.tgz" "$temporary_dir/candidate.tgz" "ec2-user@$target_public:$remote_root/"
scp "${scp_args[@]}" "$temporary_dir/workload.js" "ec2-user@$load_public:$remote_root/workload/"

go_builder='golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36'
k6_image='grafana/k6:2.2.0@sha256:9bd01d6941fca969cb61bb57d2da5ee9b385fe2aa8881df3798c196564d6ace6'

ssh "${ssh_args[@]}" "ec2-user@$target_public" bash -s -- "$remote_root" "$safe_set" "$go_builder" <<'REMOTE_TARGET_BUILD'
set -euo pipefail
remote_root=$1
safe_set=$2
go_builder=$3
tar -xzf "$remote_root/champion.tgz" -C "$remote_root/champion"
tar -xzf "$remote_root/candidate.tgz" -C "$remote_root/candidate"
for stage in champion candidate; do
  docker run --rm -e GODEBUG=cpu.all=off -v "$remote_root/$stage:/src:ro" -w /src "$go_builder" go test ./...
  docker build -t "obsidio-remote-$stage:$safe_set" "$remote_root/$stage"
done
REMOTE_TARGET_BUILD

ssh "${ssh_args[@]}" "ec2-user@$load_public" "docker pull '$k6_image' >/dev/null"
k6_version=$(ssh "${ssh_args[@]}" "ec2-user@$load_public" "docker run --rm '$k6_image' version" | tr -d '\r')

count_a=0
count_b=0
for member in $sequence; do
  case "$member" in
    A)
      stage=champion
      source_sha=$champion_sha
      image_name="obsidio-remote-champion:$safe_set"
      count_a=$((count_a + 1))
      run_id="${run_prefix}-a${count_a}"
      ;;
    B)
      stage=candidate
      source_sha=$candidate_sha
      image_name="obsidio-remote-candidate:$safe_set"
      count_b=$((count_b + 1))
      run_id="${run_prefix}-b${count_b}"
      ;;
    *) echo "BENCH_SEQUENCE members must be A or B" >&2; exit 64 ;;
  esac
  if [[ "$profile" == "screen" ]]; then
    local_summary="$REPO_ROOT/benchmarks/results/${stage}-${run_id}-screen-summary.json"
  else
    local_summary="$REPO_ROOT/benchmarks/results/${stage}-${run_id}-summary.json"
  fi
  if [[ -e "$local_summary" ]]; then
    echo "refusing to overwrite $local_summary" >&2
    exit 73
  fi

  ssh "${ssh_args[@]}" "ec2-user@$target_public" bash -s -- "$image_name" <<'REMOTE_TARGET_RUN'
set -euo pipefail
image_name=$1
docker rm -f obsidio-remote-benchmark >/dev/null 2>&1 || true
docker run -d --name obsidio-remote-benchmark \
  --cpus=2 --memory=2g --memory-swap=2g \
  -p 8080:8080 "$image_name" >/dev/null
limits=$(docker inspect --format '{{.HostConfig.NanoCpus}} {{.HostConfig.Memory}} {{.HostConfig.MemorySwap}} {{.HostConfig.CpusetCpus}}' obsidio-remote-benchmark)
if [[ "$limits" != "2000000000 2147483648 2147483648 " ]]; then
  echo "unexpected target limits: $limits" >&2
  exit 65
fi
REMOTE_TARGET_RUN

  ready=false
  for _ in {1..30}; do
    if ssh "${ssh_args[@]}" "ec2-user@$load_public" "curl --fail --silent 'http://$target_private:8080/health' >/dev/null"; then
      ready=true
      break
    fi
    sleep 1
  done
  if [[ "$ready" != "true" ]]; then
    echo "target health check failed from load host" >&2
    exit 70
  fi

  remote_summary="$remote_root/results/${run_id}.json"
  remote_status="$remote_root/results/${run_id}.status"
  ssh "${ssh_args[@]}" "ec2-user@$load_public" bash -s -- "$k6_image" "$target_private" "$remote_summary" "$remote_status" "$remote_root/workload/workload.js" <<'REMOTE_K6'
set -u
k6_image=$1
target_private=$2
remote_summary=$3
remote_status=$4
remote_workload=$5
docker run --rm --network host \
  -e "TARGET=http://$target_private:8080" \
  -v "$(dirname "$remote_workload"):/scripts:ro" \
  -v "$(dirname "$remote_summary"):/results" \
  "$k6_image" run --quiet \
  --summary-export="/results/$(basename "$remote_summary")" \
  "/scripts/$(basename "$remote_workload")"
status=$?
printf '%s\n' "$status" >"$remote_status"
exit 0
REMOTE_K6

  scp "${scp_args[@]}" "ec2-user@$load_public:$remote_summary" "$local_summary"
  k6_status=$(ssh "${ssh_args[@]}" "ec2-user@$load_public" "cat '$remote_status'")
  image_id=$(ssh "${ssh_args[@]}" "ec2-user@$target_public" "docker image inspect --format '{{.Id}}' '$image_name'")

  BENCH_COMPARISON_SET="$comparison_set" \
  BENCH_ENVIRONMENT_ID="$environment_id" \
  BENCH_TARGET_INSTANCE_ID="$target_instance_id" \
  BENCH_LOAD_INSTANCE_ID="$load_instance_id" \
  BENCH_K6_IMAGE="$k6_image" \
  BENCH_K6_VERSION="$k6_version" \
  BENCH_REMOTE_IMAGE_NAME="$image_name" \
  node "$REPO_ROOT/benchmarks/record-remote-result.mjs" \
    "$stage" "$profile" "$run_id" "$local_summary" "$source_sha" "$image_id" "$k6_status" "$workload"
done
