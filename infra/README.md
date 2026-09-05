# Deploying cv-backend to EC2

This document describes what infrastructure the backend needs, how to run it on
EC2, and how to structure the Terraform that provisions it.

## What the app actually needs (from the code)

| Dependency | Where it's used | Managed AWS equivalent |
|---|---|---|
| PostgreSQL | `internal/database`, sqlc, goose migrations | **RDS for PostgreSQL** |
| Redis | **two jobs**: auth/OTP/session store (`internal/auth/redis_store.go`) *and* the async job queue (`hibiken/asynq` in `internal/queue`) | **ElastiCache for Redis** |
| SES | `internal/aws/ses.go`, worker sends email via asynq | **SES** (domain identity + production access) |
| S3 | image uploads — item images, borrowing images, group logos (`internal/image`, `scripts/object-storage`) | **S3 bucket** |
| HTTP `:8080` | chi server in `cmd/main.go`, health endpoint at `internal/api/health.go` | **ALB** or reverse proxy |
| Background worker | asynq worker — currently started **in-process** by `cmd/main.go`; standalone binary also exists at `scripts/worker/main.go` | runs inside the same container for now |
| goose migrations | `scripts/docker-entrypoint.sh` runs `goose up` on container start | release step |

### Details the code implies

- **Redis is stateful here.** If ElastiCache is wiped, you lose queued emails
  *and* active sessions/OTPs. Enable snapshots/AOF; don't treat it as a
  throwaway cache.
- **asynq does not support Redis Cluster mode.** Use a cluster-mode-disabled
  ElastiCache replication group (or a single node).
- **Drop the static AWS keys in prod.** `LoadAWSConfig` falls back to the default
  credential chain when `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` are empty —
  so attach an **IAM instance role** to the EC2 and leave those env vars unset.
  LocalStack-only code paths key off `AWS_ENDPOINT_URL`, so leave that unset too.
- **Migrations run on boot.** Fine for one instance. If you ever run more than
  one, migrations racing on startup is a problem — move `goose up` to a one-shot
  deploy step.
- `CORS_ALLOWED_ORIGINS` and `JWT_SIGNING_KEY` must be set to real prod values;
  the sample defaults are dev-only.

## Full component checklist

### Compute
- EC2 instance (start `t3.small`, Amazon Linux 2023 or Ubuntu). Run the existing
  Docker image via `docker compose` on the host or a systemd unit.
- Elastic IP (stable address) *or* an Auto Scaling Group of size 1 + ALB for
  self-healing.
- IAM instance profile: least-privilege policy for `ses:SendEmail*` and the
  specific S3 bucket ARNs.

### Networking
- VPC with public + private subnets across 2 AZs.
- EC2 in a public subnet with a tight security group (cheapest — avoids a
  ~$32/mo NAT gateway). RDS + ElastiCache in private subnets.
- Security groups, chained: ALB SG (443/80 from world) → EC2 SG (8080 from ALB
  SG) → RDS SG (5432 from EC2 SG) → Redis SG (6379 from EC2 SG).
- SSM Session Manager for shell access instead of opening port 22.

### Edge / TLS
- ALB + ACM certificate, target group health check pointed at `/health` (or
  whatever `health.go` exposes).
- Cheaper alternative: Caddy/nginx on the box with Let's Encrypt, no ALB.
- Route 53 hosted zone for the domain.

### Data
- RDS PostgreSQL: `db.t4g.micro`, automated backups on, storage autoscaling,
  subnet group + parameter group. Take a manual snapshot before each deploy that
  includes migrations.
- ElastiCache Redis: `cache.t4g.micro`, single node to start, snapshots enabled.

### Email (SES)
- Verify the **domain**, not a single address: DKIM CNAMEs, SPF, DMARC records in
  Route 53.
- Request production access (moves you out of the sandbox so you can email
  unverified recipients).
- Configuration set → SNS topic for bounces/complaints.
- Confirm SES is available in your chosen region and that it matches
  `AWS_REGION`.

### Object storage (S3)
- Bucket with Block Public Access on; serve images via presigned URLs (code
  already does this) or CloudFront.
- Lifecycle rules, versioning, bucket CORS config if the frontend uploads
  directly.

### Secrets & config
- SSM Parameter Store (free) or Secrets Manager for: `JWT_SIGNING_KEY`, DB
  password / `GOOSE_DBSTRING`, `REDIS_PASSWORD`.
- Entrypoint or cloud-init pulls them into the container env. Never ship a
  populated `.env` to the box.

### Deploy pipeline
- ECR for the image. GitHub Actions builds + pushes on merge, then triggers
  `docker compose pull && up -d` on the box via SSM Run Command.
- EC2 user-data / cloud-init: install Docker, `aws ecr get-login`, pull compose
  file, start.

### Observability
- CloudWatch Logs via the `awslogs` Docker log driver (app already logs JSON to
  stdout).
- Alarms: EC2 status check, RDS free storage + connections, ElastiCache memory +
  evictions, ALB 5xx, SES bounce/complaint rate.
- Optional: asynqmon UI behind auth for queue visibility.

## Minimal vs. proper

- **Cheapest:** one EC2 box running the current `docker-compose.yml` (Postgres +
  Redis + app) with Caddy for TLS. ~$15–20/mo. You own backups and lose data if
  the box dies.
- **Recommended: single EC2 + RDS** (see below). ~$30–35/mo. Cheap compute, but
  the important data is managed and backed up by AWS.
- **Proper:** EC2 + RDS + ElastiCache + ALB + S3 + SES. ~$60–90/mo, managed
  backups and failure isolation.

## Recommended: single EC2 + RDS

Keep one cheap box for compute and Redis; let AWS own the data that can't be
lost.

### Layout

- **One EC2 instance** (`t3.small`, or `t3.micro` if tight) in a public subnet,
  running `docker compose` with:
  - `app` — HTTP server + in-process asynq worker
  - `redis` — with `appendonly yes` and the existing `redisdata` volume
  - `caddy` — TLS via Let's Encrypt, reverse-proxies to `app:8080`
- **RDS PostgreSQL** (`db.t4g.micro`, single-AZ), `publicly_accessible = false`,
  same VPC.

### Why this covers "data cannot be lost"

RDS gives you, out of the box:

- Automated daily backups + point-in-time recovery (set retention to 7–35 days)
- Deletion protection + a final snapshot on teardown
- Managed patching, storage autoscaling, restore-to-new-instance on corruption

Single-AZ is about *availability* (an AZ outage = minutes–hours of downtime,
rare), not *durability* — the data survives regardless. Multi-AZ ~doubles RDS
cost for automatic failover; skip it now, enable later if uptime matters.

Redis on the box is fine: it only holds sessions, OTPs, and queued emails. If the
box dies, users re-login and a few notification emails don't send. Nothing
permanent.

### Rough cost

| Item | Monthly |
|---|---|
| EC2 `t3.small` | ~$15 (~$7.50 for `t3.micro`) |
| RDS `db.t4g.micro` + 20 GB gp3 | ~$14 |
| EIP, S3, SES, transfer | ~$1–2 |
| **Total** | **~$30–35** |

### Config / code changes

- `POSTGRES_HOST` → RDS endpoint; `POSTGRES_SSL_MODE=require` (ideally
  `verify-full` with the RDS CA bundle).
- `GOOSE_DBSTRING` →
  `postgres://USER:PASS@<rds-endpoint>:5432/campus_vault?sslmode=require`.
- DB password lives in SSM Parameter Store, injected into the container at start —
  not in a committed `.env`.
- Remove the `db` service from the box's compose file; add `caddy`.
- `scripts/docker-entrypoint.sh` waits on host `db` (`>/dev/tcp/db/5432`) — point
  it at the RDS host or drop the loop and let `goose` retry.
- Before any deploy with schema changes, take a manual RDS snapshot (or automate
  it in CI).

### Security groups

- EC2 SG: inbound 80/443 from anywhere; no port 22 (use SSM Session Manager).
- RDS SG: inbound 5432 **from the EC2 SG only**; `publicly_accessible = false`.

### Terraform impact

The "proper" layout minus two files: drop `redis.tf` (Redis is on the box) and
`edge.tf` (Caddy replaces the ALB). Keep `network.tf`, `rds.tf`, `ec2.tf`,
`s3.tf`, `ses.tf`, `dns.tf`, `ssm.tf`.

## Terraform

For reproducible AWS infra with this many moving parts, Terraform is the right
call. Manual console setup won't be repeatable and it's easy to forget half the
security-group rules.

### Structure — `infra/` folder in this repo

Keeping it in the same repo is fine and convenient for a small team. Start
**flat, file-per-concern**; extract modules only when you add a second
environment.

```
infra/
  terraform/
    backend.tf          # S3 remote state + DynamoDB lock
    versions.tf         # provider + required_version pins
    variables.tf
    outputs.tf
    network.tf          # vpc, subnets, igw, route tables, security groups
    rds.tf              # db instance, subnet group, parameter group
    redis.tf            # elasticache replication group (cluster mode disabled)
    s3.tf               # image bucket + policy
    ses.tf              # domain identity, dkim, config set, SNS topic
    ec2.tf              # launch template, EIP or ASG, IAM instance profile + policy
    edge.tf             # alb, target group, acm cert, listener
    dns.tf              # route53 zone + records (app, dkim, spf, dmarc)
    ssm.tf              # parameter resources (values set out-of-band)
    terraform.tfvars    # non-secret values; gitignore if it holds anything sensitive
  bootstrap/            # tiny separate config w/ local state: creates the state bucket + lock table
  README.md             # this file
```

When you need `dev` + `prod`, move the resource files into
`modules/{network,database,cache,compute,edge}/` and add thin
`environments/dev/` and `environments/prod/` roots that call them with different
`tfvars`.

### What stays out of Terraform

- Docker image build/push — that's CI's job.
- Secret *values* — Terraform creates the SSM parameter *resource* with a
  placeholder and `lifecycle { ignore_changes = [value] }`; you set the real
  value with `aws ssm put-parameter` once.
- Running migrations — deploy-time step, not infra.
- Remote state backend bootstrap — chicken-and-egg, so the `bootstrap/` config
  uses local state and is applied once.
