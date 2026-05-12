# EC2 + Cloudflare Quick Tunnel Deployment

This guide deploys the Go API on an EC2 instance, connects it to Neon Postgres, and exposes it through a temporary Cloudflare Tunnel URL. It is meant for MVP testing before buying a domain.

## Architecture

```text
Cloudflare Pages web app -> trycloudflare.com URL -> cloudflared on EC2 -> Go API container -> Neon Postgres
```

The production Compose file does not run Postgres. Neon is the production database.

## 1. Prepare Neon

1. Create a Neon project and database.
2. Copy the pooled Postgres connection string.
3. Make sure the URL ends with `sslmode=require`.

Example:

```env
DATABASE_URL=postgres://USER:PASSWORD@HOST.neon.tech/DBNAME?sslmode=require
```

## 2. Prepare EC2

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

## 3. Copy The App

```bash
git clone https://github.com/kuo52033/vocab_review.git
cd vocab_review
cp .env.production.example .env.production
```

Edit `.env.production`:

```bash
nano .env.production
```

Set:

```env
DATABASE_URL=postgres://USER:PASSWORD@HOST.neon.tech/DBNAME?sslmode=require
LOG_COLOR=false
VOCAB_ENRICHMENT_BASE_URL=https://api.openai.com/v1
VOCAB_ENRICHMENT_API_KEY=your_openai_key_if_using_autocomplete
VOCAB_ENRICHMENT_MODEL=gpt-4.1-mini
```

Do not commit `.env.production`.

## 4. Run Migrations

```bash
make prod-migrate
```

This applies `backend/migrations` to Neon.

## 5. Start API And Tunnel

```bash
make prod-build
make prod-up
make prod-logs
```

In the logs, find the Cloudflare quick tunnel URL:

```text
https://something.trycloudflare.com
```

Test it:

```bash
curl -i https://something.trycloudflare.com/healthz
```

Expected:

```text
HTTP/2 200
```

## 6. Connect Cloudflare Pages

In Cloudflare Pages, set the web build environment variable:

```env
VITE_API_URL=https://something.trycloudflare.com
```

Build command:

```bash
npm run build:web
```

Build output directory:

```text
apps/web/dist
```

If Cloudflare Pages asks for a deploy command, use:

```bash
npx wrangler pages deploy apps/web/dist
```

Do not use plain `npx wrangler deploy` from the repository root. This is a monorepo, and Wrangler may fail because it cannot detect which workspace app to deploy.

Redeploy the Pages site after setting the variable.

## 7. Operations

Restart production services:

```bash
make prod-down
make prod-up
```

Tail logs:

```bash
make prod-logs
```

Run migrations after pulling new backend changes:

```bash
git pull
make prod-migrate
make prod-build
make prod-up
```

## Limitations

- Quick tunnel URLs are temporary and may change after restart.
- Use a named Cloudflare Tunnel when you want a stable API URL.
- The notification worker is built into the image but not started in this first production Compose file.
