#!/usr/bin/env bash

set -euo pipefail

AWS_BENCH_DIR=$(cd "$(dirname "$0")" && pwd)
source "$AWS_BENCH_DIR/lib.sh"

require_command curl
require_aws_identity

if [[ "${AWS_CONFIRM:-}" != "create-paid-resources" ]]; then
  echo "This command creates billable AWS resources." >&2
  echo "Re-run with AWS_CONFIRM=create-paid-resources after reviewing README.md." >&2
  exit 64
fi

mkdir -p "$AWS_BENCH_STATE_DIR"
chmod 700 "$AWS_BENCH_STATE_DIR"

target_type=${AWS_TARGET_INSTANCE_TYPE:-c7i.xlarge}
load_type=${AWS_LOAD_INSTANCE_TYPE:-c7i.large}
shutdown_minutes=${AWS_SHUTDOWN_AFTER_MINUTES:-240}
budget_email=${AWS_BUDGET_EMAIL:-}
monthly_budget_usd=${AWS_MONTHLY_BUDGET_USD:-20}
admin_cidr=${AWS_ADMIN_CIDR:-}

if [[ -z "$budget_email" ]]; then
  echo "Set AWS_BUDGET_EMAIL to the address that should receive cost alerts." >&2
  exit 64
fi

if [[ -z "$admin_cidr" ]]; then
  public_ip=$(curl --fail --silent --show-error https://checkip.amazonaws.com | tr -d '[:space:]')
  admin_cidr="${public_ip}/32"
fi

availability_zone=${AWS_AVAILABILITY_ZONE:-}
if [[ -z "$availability_zone" ]]; then
  availability_zone=$(aws ec2 describe-instance-type-offerings \
    --region "$AWS_BENCH_REGION" \
    --location-type availability-zone \
    --filters "Name=instance-type,Values=$target_type" \
    --query 'InstanceTypeOfferings[].Location' \
    --output text | tr '\t' '\n' | sort | head -n 1)
fi

if [[ -z "$availability_zone" ]]; then
  echo "No $target_type offering found in $AWS_BENCH_REGION." >&2
  exit 69
fi

load_available=$(aws ec2 describe-instance-type-offerings \
  --region "$AWS_BENCH_REGION" \
  --location-type availability-zone \
  --filters "Name=instance-type,Values=$load_type" "Name=location,Values=$availability_zone" \
  --query 'length(InstanceTypeOfferings)' \
  --output text)
if [[ "$load_available" != "1" ]]; then
  echo "$load_type is not offered in $availability_zone." >&2
  exit 69
fi

if [[ ! -f "$AWS_BENCH_KEY_PATH" ]]; then
  existing_key=$(aws ec2 describe-key-pairs \
    --region "$AWS_BENCH_REGION" \
    --key-names "$AWS_BENCH_KEY_NAME" \
    --query 'KeyPairs[0].KeyName' \
    --output text 2>/dev/null || true)
  if [[ "$existing_key" == "$AWS_BENCH_KEY_NAME" ]]; then
    echo "AWS key $AWS_BENCH_KEY_NAME exists but its local private key is missing." >&2
    echo "Choose a new AWS_STACK or remove that unused AWS key explicitly." >&2
    exit 73
  fi
  aws ec2 create-key-pair \
    --region "$AWS_BENCH_REGION" \
    --key-name "$AWS_BENCH_KEY_NAME" \
    --key-type ed25519 \
    --query KeyMaterial \
    --output text >"$AWS_BENCH_KEY_PATH"
  chmod 600 "$AWS_BENCH_KEY_PATH"
fi

echo "Provisioning $AWS_BENCH_STACK in $availability_zone"
echo "  target: $target_type"
echo "  load:   $load_type"
echo "  SSH:    $admin_cidr"
echo "  safety: both instances stop after $shutdown_minutes minutes"
echo "  budget: account-wide USD $monthly_budget_usd/month alerts to $budget_email"

aws cloudformation deploy \
  --region "$AWS_BENCH_REGION" \
  --stack-name "$AWS_BENCH_STACK" \
  --template-file "$AWS_BENCH_DIR/cloudformation.yaml" \
  --no-fail-on-empty-changeset \
  --parameter-overrides \
    "AdminCidr=$admin_cidr" \
    "AvailabilityZone=$availability_zone" \
    "KeyName=$AWS_BENCH_KEY_NAME" \
    "TargetInstanceType=$target_type" \
    "LoadInstanceType=$load_type" \
    "ShutdownAfterMinutes=$shutdown_minutes" \
    "BudgetEmail=$budget_email" \
    "MonthlyBudgetUsd=$monthly_budget_usd" \
  --tags Project=ObsidioBench ManagedBy=CloudFormation

aws cloudformation describe-stacks \
  --region "$AWS_BENCH_REGION" \
  --stack-name "$AWS_BENCH_STACK" \
  --query 'Stacks[0].Outputs' \
  --output json >"$AWS_BENCH_OUTPUTS_PATH"

"$AWS_BENCH_DIR/verify.sh"
