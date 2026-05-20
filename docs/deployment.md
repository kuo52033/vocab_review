# Production Deployment

Production deploys are handled by GitHub Actions and AWS SSM. The EC2 instance does not pull the repository. GitHub Actions builds the backend Docker image, pushes it to ECR, sends only the `deploy/` directory to EC2, updates `BACKEND_IMAGE` in the existing production environment file, runs migrations, and restarts Docker Compose.

## Runtime Layout

The EC2 instance stores runtime files under:

```text
/opt/vocab-review/
  .env.production
  Caddyfile
  docker-compose.prod.yml
```

`.env.production` must be created manually on EC2 and is never committed. It must include production settings such as `DATABASE_URL`. The deploy workflow will add or replace only the `BACKEND_IMAGE=...` line.

## GitHub Configuration

Create these GitHub Actions values before enabling deployment:

- Secret `AWS_ROLE_TO_ASSUME`: IAM role ARN trusted by GitHub Actions OIDC.
- Variable `AWS_REGION`: AWS region for ECR, SSM, and EC2.
- Variable `AWS_ACCOUNT_ID`: AWS account ID that owns the ECR repository and EC2 instance.
- Variable `ECR_REPOSITORY`: ECR repository name for the backend image.
- Variable `EC2_INSTANCE_ID`: target EC2 instance ID.
- Variable `DEPLOY_DIR`: target directory on EC2, normally `/opt/vocab-review`.

The deployment workflow runs on pushes to `main`.

## EC2 Requirements

Attach an IAM role to the EC2 instance with:

- `AmazonSSMManagedInstanceCore`
- `AmazonEC2ContainerRegistryReadOnly`

The instance also needs:

- Docker Engine with the Docker Compose plugin available as `docker compose`.
- AWS CLI available to the SSM shell command.
- `/opt/vocab-review/.env.production` already present.

The production env file should include required backend settings, for example:

```sh
DATABASE_URL=postgres://...
```

Do not commit `.env.production`, and do not hardcode `DATABASE_URL` in workflow or compose files.

## GitHub Actions AWS Role

The GitHub OIDC role needs permission to push to ECR and run SSM commands against the target instance.

At minimum, grant ECR push permissions for the backend repository:

- `ecr:GetAuthorizationToken`
- `ecr:BatchCheckLayerAvailability`
- `ecr:InitiateLayerUpload`
- `ecr:UploadLayerPart`
- `ecr:CompleteLayerUpload`
- `ecr:PutImage`

Grant SSM command permissions:

- `ssm:SendCommand`
- `ssm:GetCommandInvocation`

Scope `ssm:SendCommand` to:

- the target EC2 instance ARN
- the `AWS-RunShellScript` SSM document ARN

## Networking

The EC2 security group should allow:

- inbound TCP `80`
- inbound TCP `443`

No inbound SSH rule is required when SSM access is working.

## Cloudflare DNS

Create a Cloudflare DNS record:

- Type: `A`
- Name: `api.vocabreview.uk`
- Value: EC2 Elastic IP
- Proxy status: Proxied

Caddy serves `api.vocabreview.uk` and reverse-proxies traffic to the backend API service at `api:8080`.

## Deploy Flow

On push to `main`, `.github/workflows/deploy.yml`:

1. Checks out the repository in GitHub Actions.
2. Assumes the AWS role through GitHub OIDC.
3. Logs in to ECR.
4. Builds `backend/Dockerfile`.
5. Tags the image as `main-${GITHUB_SHA::12}`.
6. Pushes the image to ECR.
7. Packages only the `deploy/` directory.
8. Sends an SSM command to EC2 that:
   - creates `/opt/vocab-review`
   - extracts the deploy files
   - preserves `/opt/vocab-review/.env.production`
   - updates `BACKEND_IMAGE=...`
   - logs in to ECR from EC2
   - runs `docker compose --env-file .env.production -f docker-compose.prod.yml pull`
   - runs `docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate`
   - runs `docker compose --env-file .env.production -f docker-compose.prod.yml up -d`
