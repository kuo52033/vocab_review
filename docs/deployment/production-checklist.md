# Production Deployment Checklist

Use this checklist before changing `vocabreview.uk` production. Keep secrets in `.env.production` on EC2 only.

## Before Deploy

- Confirm GitHub Actions pushed the intended backend image tag to ECR.
- Confirm `BACKEND_IMAGE` in `/opt/vocab-review/.env.production` points to that exact tag.
- Confirm `DATABASE_URL` uses the production Postgres database and includes `sslmode=require`.
- Confirm Cloudflare Pages has `VITE_API_URL=https://api.vocabreview.uk`.
- Confirm EC2 security group exposes only `80`, `443`, and SSH from your own IP.
- Confirm `api.vocabreview.uk` DNS points to the EC2 public IP.
- If Cloudflare proxy is enabled, confirm SSL/TLS mode is **Full (strict)**.
- Confirm `docker-compose.prod.yml`, `Caddyfile`, and `Makefile` on EC2 match the repository version.

## Deploy

Run on EC2 from `/opt/vocab-review`:

```bash
aws ecr get-login-password --region ap-northeast-1 | docker login --username AWS --password-stdin 293133628661.dkr.ecr.ap-northeast-1.amazonaws.com
make prod-pull
make prod-migrate
make prod-up
```

Then redeploy Cloudflare Pages if the web build or `VITE_API_URL` changed.

## Verify

- Run `curl -i https://api.vocabreview.uk/healthz` and expect `HTTP/2 200`.
- If Cloudflare proxy was changed, run the health check again after the DNS record shows **Proxied**.
- Open `https://vocabreview.uk` and sign in.
- Create one test vocab card.
- Start Review if a due card exists.
- Open Active cards and confirm the new card appears.
- Check backend logs with `make prod-logs` and confirm there are no repeated `4xx` or `5xx` errors.

## Rollback

- Change `BACKEND_IMAGE` in `.env.production` back to the previous working tag.
- Run `make prod-pull` and `make prod-up`.
- Recheck `https://api.vocabreview.uk/healthz`.
- If a migration caused the issue, stop and inspect the database before manually changing schema or data.

## After Deploy

- Record the deployed image tag and timestamp.
- Keep Cloudflare proxy in **Full (strict)** mode if proxy is enabled.
- Do not commit `.env.production`, API keys, database URLs, or Cloudflare tokens.
