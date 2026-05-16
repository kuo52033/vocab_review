# EC2 + Caddy Domain Deployment

This guide deploys the Go API on an EC2 instance, connects it to managed Postgres, and exposes it at `https://api.vocabreview.uk` through Caddy. The web app should stay on Cloudflare Pages at `vocabreview.uk` or `www.vocabreview.uk`.

For repeat deploys, use [Production Deployment Checklist](production-checklist.md).

## Architecture

```text
Cloudflare Pages web app -> https://api.vocabreview.uk -> Caddy on EC2 -> Go API container -> managed Postgres
```

The production Compose file does not run Postgres. Use Supabase, Neon, or another managed Postgres provider.

## 1. Prepare Postgres

Create a production Postgres database and copy the connection string.

For Supabase, use the pooler connection string for the running API when possible, and include `sslmode=require`:

```env
DATABASE_URL=postgresql://USER:PASSWORD@HOST:PORT/postgres?sslmode=require
```

If migrations fail through a pooler, run migrations with the direct database connection string, then switch `.env.production` back to the pooled connection for the API.

## 2. Prepare DNS

In Cloudflare DNS, create this record:

```text
Type: A
Name: api
Content: <EC2_PUBLIC_IP>
Proxy status: DNS only
TTL: Auto
```

Start with DNS-only while Caddy gets its first certificate. After `https://api.vocabreview.uk/healthz` works, you can enable the Cloudflare proxy and set SSL/TLS mode to **Full (strict)**.

To enable Cloudflare proxy after HTTPS works:

1. Go to Cloudflare DNS.
2. Change the `api` record from **DNS only** to **Proxied**.
3. Go to **SSL/TLS** and set encryption mode to **Full (strict)**.
4. Recheck `https://api.vocabreview.uk/healthz`.

Do not use **Flexible** SSL mode. Flexible makes Cloudflare connect to EC2 over plain HTTP and can cause redirect loops or weaker transport security.

## 3. Prepare EC2

Use Ubuntu on a small EC2 instance. Install Docker, the Docker Compose plugin, Git, and Make:

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl git make
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker "$USER"
```

Log out and back in so the `docker` group applies.

Open these inbound ports in the EC2 security group:

```text
TCP 80  from 0.0.0.0/0
TCP 443 from 0.0.0.0/0
```

Do not expose port `8080` publicly. The API is reachable only inside the Docker network; Caddy is the public entry point.

Attach an IAM role to the EC2 instance so it can pull images from ECR without storing AWS access keys on the server. Use this minimum policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecr:GetAuthorizationToken",
        "ecr:BatchCheckLayerAvailability",
        "ecr:GetDownloadUrlForLayer",
        "ecr:BatchGetImage"
      ],
      "Resource": "*"
    }
  ]
}
```

Install the AWS CLI:

```bash
sudo apt-get install -y unzip
curl "https://awscli.amazonaws.com/awscli-exe-linux-$(uname -m).zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install
```

Do not run `aws configure` on EC2. Verify the instance role:

```bash
aws sts get-caller-identity
```

## 4. Prepare ECR And GitHub Actions

Create an ECR repository if it does not exist:

```bash
aws ecr create-repository --repository-name vocab-review --region ap-northeast-1
```

The GitHub Actions workflow uses OIDC to assume this role:

```text
arn:aws:iam::293133628661:role/vocab-review-github-actions
```

The workflow pushes a Docker image to ECR when code is pushed to `master`, or when manually run with a custom `docker_tag`.

## 5. Prepare Deployment Files

EC2 only needs:

```text
/opt/vocab-review/
├── Caddyfile
├── Makefile
├── docker-compose.prod.yml
└── .env.production
```

Copy `docker-compose.prod.yml`, `Caddyfile`, and `Makefile` from this repository to `/opt/vocab-review/`.

Create `.env.production`:

```bash
sudo mkdir -p /opt/vocab-review
sudo chown "$USER":"$USER" /opt/vocab-review
cd /opt/vocab-review
nano .env.production
```

Set:

```env
BACKEND_IMAGE=293133628661.dkr.ecr.ap-northeast-1.amazonaws.com/vocab-review:master-SHORT_SHA
DATABASE_URL=postgresql://USER:PASSWORD@HOST:PORT/postgres?sslmode=require
LOG_COLOR=false
VOCAB_ENRICHMENT_BASE_URL=https://api.openai.com/v1
VOCAB_ENRICHMENT_API_KEY=your_openai_key_if_using_autocomplete
VOCAB_ENRICHMENT_MODEL=gpt-4.1-mini
```

Do not commit `.env.production`.

## 6. Pull Image, Run Migrations, And Start API

Log in to ECR using the EC2 role:

```bash
aws ecr get-login-password --region ap-northeast-1 | docker login --username AWS --password-stdin 293133628661.dkr.ecr.ap-northeast-1.amazonaws.com
```

From `/opt/vocab-review`:

```bash
make prod-pull
make prod-migrate
make prod-up
make prod-logs
```

Or without Make:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml pull
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
docker compose --env-file .env.production -f docker-compose.prod.yml logs -f
```

Caddy stores certificates in Docker volumes `caddy_data` and `caddy_config`, so certs survive container restarts.

Test:

```bash
curl -i https://api.vocabreview.uk/healthz
```

Expected:

```text
HTTP/2 200
```

## 7. Connect Cloudflare Pages

In Cloudflare Pages, set:

```env
VITE_API_URL=https://api.vocabreview.uk
```

Build command:

```bash
npm run build:web
```

Build output directory:

```text
apps/web/dist
```

Redeploy the Pages site after changing `VITE_API_URL`.

## 8. Operations

Deploy a new backend image:

```bash
cd /opt/vocab-review
nano .env.production
aws ecr get-login-password --region ap-northeast-1 | docker login --username AWS --password-stdin 293133628661.dkr.ecr.ap-northeast-1.amazonaws.com
make prod-pull
make prod-migrate
make prod-up
```

Update `BACKEND_IMAGE` in `.env.production` to the exact tag printed by GitHub Actions.

Restart production services:

```bash
make prod-down
make prod-up
```

Tail logs:

```bash
make prod-logs
```

If `docker-compose.prod.yml`, `Caddyfile`, or `Makefile` changes, copy the updated file to `/opt/vocab-review/` before restarting.

## Notes

- `api.vocabreview.uk` starts DNS-only for simpler certificate setup.
- After HTTPS works, Cloudflare proxy is optional. If enabled, use **Full (strict)** SSL/TLS mode.
- After proxy is enabled, Cloudflare hides the EC2 public IP from normal DNS lookups, but the EC2 security group still controls who can connect to ports `80` and `443`.
- The notification worker is built into the image but not started in this first production Compose file.
