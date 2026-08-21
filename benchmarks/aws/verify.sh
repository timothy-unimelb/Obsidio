#!/usr/bin/env bash

set -euo pipefail

AWS_BENCH_DIR=$(cd "$(dirname "$0")" && pwd)
source "$AWS_BENCH_DIR/lib.sh"

require_aws_identity
require_command ssh

if [[ ! -f "$AWS_BENCH_KEY_PATH" ]]; then
  echo "Private key not found: $AWS_BENCH_KEY_PATH" >&2
  exit 66
fi

target_public=$(stack_output TargetPublicIp)
target_private=$(stack_output TargetPrivateIp)
load_public=$(stack_output LoadPublicIp)
environment_id=$(stack_output EnvironmentId)
ssh_args=(
  -i "$AWS_BENCH_KEY_PATH"
  -o "UserKnownHostsFile=$AWS_BENCH_KNOWN_HOSTS"
  -o StrictHostKeyChecking=accept-new
  -o ConnectTimeout=10
)

for host in "$target_public" "$load_public"; do
  ready=false
  for _ in {1..30}; do
    if ssh "${ssh_args[@]}" "ec2-user@$host" 'cloud-init status --wait >/dev/null && docker version >/dev/null' 2>/dev/null; then
      ready=true
      break
    fi
    sleep 10
  done
  if [[ "$ready" != "true" ]]; then
    echo "Host $host did not become ready within five minutes." >&2
    exit 70
  fi
done

target_arch=$(ssh "${ssh_args[@]}" "ec2-user@$target_public" 'uname -m')
load_arch=$(ssh "${ssh_args[@]}" "ec2-user@$load_public" 'uname -m')
target_cpus=$(ssh "${ssh_args[@]}" "ec2-user@$target_public" 'nproc')
load_cpus=$(ssh "${ssh_args[@]}" "ec2-user@$load_public" 'nproc')

if [[ "$target_arch" != "x86_64" || "$load_arch" != "x86_64" ]]; then
  echo "Expected x86_64 hosts; got target=$target_arch load=$load_arch" >&2
  exit 65
fi
if (( target_cpus <= 2 )); then
  echo "Target must expose more than two vCPUs so the cgroup quota—not host size—is tested." >&2
  exit 65
fi

cat <<EOF
AWS benchmark environment is ready.
  environment:    $environment_id
  target:         $target_public ($target_private), $target_cpus visible CPUs, $target_arch
  load generator: $load_public, $load_cpus CPUs, $load_arch
  target API:     private port 8080, reachable only from the load security group

Paid resources are running. Destroy them when finished:
  ./benchmarks/aws/destroy.sh
EOF
