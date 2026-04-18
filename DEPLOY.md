# Production Deployment Guide

## Prerequisites
- Docker and Docker Compose installed
- Domain name with DNS configured
- SSL certificate (Let's Encrypt recommended)

## Quick Start

### 1. Clone and configure
```bash
git clone <repo-url>
cd robot-remote-maint
cp .env.example .env
# Edit .env with your production values
```

### 2. Build and start
```bash
docker compose -f docker-compose.prod.yml up -d
```

### 3. Check status
```bash
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f api
```

### 4. Stop
```bash
docker compose -f docker-compose.prod.yml down
```

## Agent Deployment
See `agent/DEPLOY.md` for detailed instructions.

## Environment Variables
See `.env.example` for all available options.

## Security Checklist
- [ ] JWT_SECRET is a strong random value
- [ ] DB_PASSWORD is strong and unique
- [ ] HTTPS is enabled (reverse proxy or Caddy)
- [ ] CORS_ORIGINS is set to your domain
- [ ] Firewall rules are configured
