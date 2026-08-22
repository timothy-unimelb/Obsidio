# AWS separated benchmark environment

This directory provisions the stable x86-64 reference environment defined in
`../PROTOCOL.md`. It is intended to distinguish small optimizations that local
same-host Apple Silicon testing cannot resolve. It does not predict the judge's
absolute score or CPU model.

## Shape

- one On-Demand, non-flex `c7i.xlarge` target host;
- one On-Demand, non-flex `c7i.large` load-generator host;
- both in one subnet and availability zone, with benchmark traffic over the
  private address;
- target port 8080 reachable only from the load host's security group;
- SSH restricted to the provisioning workstation's current public IPv4 `/32`;
- Amazon Linux 2023 x86-64, encrypted 16 GiB gp3 roots, IMDSv2 required; and
- instance-initiated automatic stop after four hours by default.

The target is intentionally larger than two vCPUs. The submitted container—not
the host—is capped to `--cpus=2 --memory=2g --memory-swap=2g`, matching the known
judge budget while leaving host processors visible.

## Prerequisites

Install AWS CLI v2 and authenticate a non-root AWS identity with permission for
CloudFormation, EC2, and the small VPC created here:

```sh
aws configure sso
aws sts get-caller-identity
```

The scripts never store AWS credentials in the repository. They create one
ephemeral EC2 SSH key under ignored `.state/` and delete its AWS and local copies
with the stack.

## Provision

```sh
AWS_CONFIRM=create-paid-resources \
AWS_BUDGET_EMAIL=you@example.com \
./benchmarks/aws/provision.sh
```

Defaults:

```text
AWS_REGION=ap-southeast-2
AWS_STACK=obsidio-bench
AWS_TARGET_INSTANCE_TYPE=c7i.xlarge
AWS_LOAD_INSTANCE_TYPE=c7i.large
AWS_SHUTDOWN_AFTER_MINUTES=240
AWS_MONTHLY_BUDGET_USD=20
```

Set `AWS_ADMIN_CIDR=x.x.x.x/32` when automatic public-IP detection is not
appropriate. Set `AWS_AVAILABILITY_ZONE` to pin an AZ; otherwise the script
chooses the first AZ offering the required non-flex C7i shapes.

The stack creates an account-wide monthly cost budget with actual-spend alerts
at 50% and 80% and a forecast alert at 100%. It is deliberately account-wide so
it does not depend on cost-allocation tags becoming active first. This means
unrelated AWS usage also contributes to the alert.

Automatic shutdown is only a backstop: stopped instances retain EBS storage,
and budget notifications are delayed rather than a hard cap. Always destroy the
stack after the experiment.

## Inspect and destroy

```sh
./benchmarks/aws/status.sh
./benchmarks/aws/destroy.sh
```

`destroy.sh` deletes the instances, VPC resources, encrypted root volumes, AWS
key pair, and ignored local private key. CloudFormation tags every resource with
`Project=ObsidioBench` for identification.

## Run a comparison

The remote runner accepts two local, committed Docker build contexts. It uploads
them independently, runs correctness tests without optional CPU acceleration,
builds each image once on the x86 target, checks the Docker limits before every
run, runs pinned k6 only on the separate load host, downloads every raw summary,
and appends remote environment metadata to the local history.

```sh
BENCH_COMPARISON_SET=pgo-screen-20260822 \
RUN_PREFIX=pgo-screen \
./benchmarks/aws/run-comparison.sh \
  /path/to/champion/submission/go \
  /path/to/candidate/submission/go \
  screen
```

Default sequences are `A B A` for either profile. Override only according to
the protocol, for example the milestone sequence:

```sh
BENCH_SEQUENCE="A B B A A B" ... ./benchmarks/aws/run-comparison.sh ... full
```

The runner pins the multi-architecture k6 2.2.0 image index by digest; Docker
selects its Linux amd64 manifest on the load host. Do not change that pin inside
a comparison set.
