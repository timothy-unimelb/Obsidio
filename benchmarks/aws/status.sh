#!/usr/bin/env bash

set -euo pipefail

AWS_BENCH_DIR=$(cd "$(dirname "$0")" && pwd)
source "$AWS_BENCH_DIR/lib.sh"

require_aws_identity

aws cloudformation describe-stacks \
  --region "$AWS_BENCH_REGION" \
  --stack-name "$AWS_BENCH_STACK" \
  --query 'Stacks[0].{Status:StackStatus,Created:CreationTime,Outputs:Outputs}' \
  --output table

instance_ids=$(aws cloudformation describe-stack-resources \
  --region "$AWS_BENCH_REGION" \
  --stack-name "$AWS_BENCH_STACK" \
  --query "StackResources[?ResourceType=='AWS::EC2::Instance'].PhysicalResourceId" \
  --output text)

aws ec2 describe-instances \
  --region "$AWS_BENCH_REGION" \
  --instance-ids $instance_ids \
  --query 'Reservations[].Instances[].{Name:Tags[?Key==`Name`]|[0].Value,Id:InstanceId,Type:InstanceType,State:State.Name,PublicIp:PublicIpAddress,PrivateIp:PrivateIpAddress,Launched:LaunchTime}' \
  --output table

