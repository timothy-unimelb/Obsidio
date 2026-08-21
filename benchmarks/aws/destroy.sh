#!/usr/bin/env bash

set -euo pipefail

AWS_BENCH_DIR=$(cd "$(dirname "$0")" && pwd)
source "$AWS_BENCH_DIR/lib.sh"

require_aws_identity

echo "Deleting CloudFormation stack $AWS_BENCH_STACK in $AWS_BENCH_REGION"
aws cloudformation delete-stack \
  --region "$AWS_BENCH_REGION" \
  --stack-name "$AWS_BENCH_STACK"
aws cloudformation wait stack-delete-complete \
  --region "$AWS_BENCH_REGION" \
  --stack-name "$AWS_BENCH_STACK"

aws ec2 delete-key-pair \
  --region "$AWS_BENCH_REGION" \
  --key-name "$AWS_BENCH_KEY_NAME" >/dev/null 2>&1 || true

rm -f "$AWS_BENCH_KEY_PATH" "$AWS_BENCH_OUTPUTS_PATH" "$AWS_BENCH_KNOWN_HOSTS"
rmdir "$AWS_BENCH_STATE_DIR" 2>/dev/null || true
echo "Stack and ephemeral SSH key deleted."

