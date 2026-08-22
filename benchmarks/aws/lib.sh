#!/usr/bin/env bash

set -euo pipefail

AWS_BENCH_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
AWS_BENCH_STATE_DIR="$AWS_BENCH_DIR/.state"
AWS_BENCH_REGION=${AWS_REGION:-ap-southeast-2}
AWS_BENCH_STACK=${AWS_BENCH_STACK:-obsidio-bench}
AWS_BENCH_KEY_NAME=${AWS_KEY_NAME:-"${AWS_BENCH_STACK}-key"}
AWS_BENCH_KEY_PATH="$AWS_BENCH_STATE_DIR/${AWS_BENCH_STACK}.pem"
AWS_BENCH_OUTPUTS_PATH="$AWS_BENCH_STATE_DIR/${AWS_BENCH_STACK}-outputs.json"
AWS_BENCH_KNOWN_HOSTS="$AWS_BENCH_STATE_DIR/known_hosts"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 69
  fi
}

require_aws_identity() {
  require_command aws
  if ! aws sts get-caller-identity --region "$AWS_BENCH_REGION" >/dev/null 2>&1; then
    echo "AWS authentication is missing or expired. Run 'aws configure sso' or 'aws configure'." >&2
    exit 77
  fi
}

stack_output() {
  local key=$1
  aws cloudformation describe-stacks \
    --region "$AWS_BENCH_REGION" \
    --stack-name "$AWS_BENCH_STACK" \
    --query "Stacks[0].Outputs[?OutputKey=='${key}'].OutputValue | [0]" \
    --output text
}

ssh_options() {
  printf '%s\n' \
    -i "$AWS_BENCH_KEY_PATH" \
    -o "UserKnownHostsFile=$AWS_BENCH_KNOWN_HOSTS" \
    -o StrictHostKeyChecking=accept-new \
    -o ConnectTimeout=10
}

