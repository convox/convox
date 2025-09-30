---
title: "Migration Guide"
draft: false
slug: migration-guide
url: /cloud/migration-guide
---

# Migration Guide

This guide helps you migrate your applications to Convox Cloud from other platforms. Whether you're coming from Heroku, Render, Railway, or even a self-hosted Convox Rack, we'll walk you through the process step by step.

## Quick Migration Overview

| From Platform | Effort Level | Key Changes | Migration Time |
|---------------|-------------|-------------|----------------|
| Heroku | Low | Procfile → convox.yml | 30 minutes |
| Render | Low | render.yaml → convox.yml | 30 minutes |
| Railway | Medium | Add Dockerfile, create convox.yml | 1 hour |
| Convox Rack | Very Low | Change CLI commands | 15 minutes |
| Docker Compose | Low | docker-compose.yml → convox.yml | 30 minutes |

## Migrating from Heroku

### Prerequisites

- Heroku CLI installed
- Application source code
- Database backups (if applicable)

### Step 1: Export Heroku Configuration

```bash
# List your Heroku apps
$ heroku apps

# Export environment variables
$ heroku config -a your-heroku-app -s > .env.heroku

# Get buildpack info
$ heroku buildpacks -a your-heroku-app
```

### Step 2: Create Dockerfile

Heroku uses buildpacks, but Convox uses Docker. Create a `Dockerfile` based on your buildpack:

**For Node.js apps:**
```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["npm", "start"]
```

**For Ruby/Rails apps:**
```dockerfile
FROM ruby:3.1-alpine
RUN apk add --no-cache build-base postgresql-dev
WORKDIR /app
COPY Gemfile Gemfile.lock ./
RUN bundle install --deployment --without development test
COPY . .
EXPOSE 3000
CMD ["bundle", "exec", "rails", "server", "-b", "0.0.0.0"]
```

**For Python apps:**
```dockerfile
FROM python:3.11-alpine
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["gunicorn", "app:app", "--bind", "0.0.0.0:8000"]
```

### Step 3: Convert Procfile to convox.yml

**Heroku Procfile:**
```
web: node server.js
worker: node worker.js
release: node migrate.js
```

**Convox convox.yml:**
```yaml
services:
  web:
    build: .
    command: node server.js
    port: 3000
    scale:
      count: 1
      cpu: 250
      memory: 512
    health: /health
    
  worker:
    build: .
    command: node worker.js
    scale:
      count: 1
      cpu: 250
      memory: 512

  migrate:
    build: .
    command: node migrate.js
    scale:
      count: 0  # Template service for migrations
```

### Step 4: Handle Add-ons

Map Heroku add-ons to Convox resources:

| Heroku Add-on | Convox Equivalent |
|---------------|-------------------|
| Heroku Postgres | `postgres` resource or external |
| Heroku Redis | `redis` resource or external |
| SendGrid | Use API with environment variables |
| Papertrail | Configure syslog or use external |
| New Relic | Add agent to Dockerfile |

**Example with database:**
```yaml
resources:
  database:
    type: postgres
    options:
      storage: 10
      
services:
  web:
    build: .
    port: 3000
    resources:
      - database
```

### Step 5: Migrate Data

```bash
# Export from Heroku Postgres
$ heroku pg:backups:capture -a your-heroku-app
$ heroku pg:backups:download -a your-heroku-app

# Import to Convox
$ convox cloud resources import database --file latest.dump -a myapp -i production
```

### Step 6: Deploy to Convox Cloud

```bash
# Create machine via Convox Console first
# Log into console.convox.com > Cloud Machines > New Machine

# Set environment variables from Heroku export
$ convox cloud env set $(cat .env.heroku) -a myapp -i production

# Deploy application
$ convox cloud deploy -i production

# Run migrations
$ convox cloud run migrate "node migrate.js" -a myapp -i production
```

### Step 7: Update DNS

Point your domain from Heroku to Convox:

```bash
# Get Convox app URL
$ convox cloud services -a myapp -i production

# Update DNS CNAME from:
# your-app.herokuapp.com
# To:
# web.myapp.cloud.convox.com
```

## Migrating from Render

### Step 1: Export Render Configuration

Your `render.yaml` defines your services. We'll convert this to `convox.yml`.

**Render render.yaml:**
```yaml
services:
  - type: web
    name: myapp
    env: node
    buildCommand: npm install
    startCommand: npm start
    envVars:
      - key: NODE_ENV
        value: production
    
  - type: worker
    name: background-worker
    env: node
    buildCommand: npm install
    startCommand: npm run worker

databases:
  - name: myapp-db
    plan: starter
```

### Step 2: Create Dockerfile

Since Render handles builds automatically, you'll need a Dockerfile:

```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["npm", "start"]
```

### Step 3: Convert to convox.yml

**Convox convox.yml:**
```yaml
environment:
  - NODE_ENV=production

resources:
  database:
    type: postgres
    options:
      storage: 10

services:
  web:
    build: .
    command: npm start
    port: 3000
    scale:
      count: 1
      cpu: 500
      memory: 512
    resources:
      - database
    
  worker:
    build: .
    command: npm run worker
    scale:
      count: 1
      cpu: 250
      memory: 512
    resources:
      - database
```

### Step 4: Migrate and Deploy

```bash
# Create Convox machine via Console
# Log into console.convox.com > Cloud Machines > New Machine

# Deploy to Convox
$ convox cloud deploy -i production

# Copy environment variables
$ convox cloud env set DATABASE_URL=postgres://... -a myapp -i production
```

## Migrating from Railway

Railway uses automatic builds and nixpacks. You'll need to be more explicit with Convox.

### Step 1: Create Dockerfile

Railway automatically detects your framework. With Convox, create an explicit Dockerfile:

```dockerfile
# Example for Next.js app
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/package*.json ./
RUN npm ci --only=production
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
EXPOSE 3000
CMD ["npm", "start"]
```

### Step 2: Create convox.yml

```yaml
services:
  web:
    build: .
    port: 3000
    scale:
      count: 1-3
      cpu: 500
      memory: 1024
      targets:
        cpu: 70
    health: /api/health
```

### Step 3: Handle Railway Variables

```bash
# Export from Railway (manually copy from dashboard)
# Then set in Convox
$ convox cloud env set \
    DATABASE_URL=postgres://... \
    REDIS_URL=redis://... \
    SECRET_KEY=... \
    -a myapp -i production
```

### Step 4: Deploy

```bash
# Create machine via Console first
$ convox cloud deploy -i production
```

## Migrating from Self-Hosted Convox Rack

Migration from a self-hosted Rack to Cloud is straightforward since the application format is the same.

### Step 1: Export Application

```bash
# From your existing rack
$ convox apps export myapp --file myapp.tgz
$ convox env -a myapp > myapp.env
```

### Step 2: Create Cloud Machine

Log into the [Convox Console](https://console.convox.com):
1. Navigate to Cloud Machines
2. Click "New Machine"
3. Select size and region
4. Create the machine

### Step 3: Import Application

```bash
# Import the app
$ convox cloud apps import myapp --file myapp.tgz -i production

# Set environment variables
$ convox cloud env set $(cat myapp.env) -a myapp -i production
```

### Step 4: Adjust for Cloud Limitations

Review your `convox.yml` for Cloud compatibility:

**Remove unsupported features:**
```yaml
# Remove these if present:
agents:        # Not supported in Cloud
volumes:       # No persistent volumes in Cloud
privileged:    # Limited in Cloud
```

**Handle persistent storage differently:**
```yaml
# Instead of volumes, use external storage
services:
  web:
    environment:
      - S3_BUCKET=myapp-uploads  # Use S3 instead of volumes
```

### Step 5: Update CI/CD

Update your deployment commands:

**Before (Rack):**
```bash
convox deploy -a myapp -r production
```

**After (Cloud):**
```bash
convox cloud deploy -a myapp -i production
```

## Migrating from Docker Compose

### Step 1: Convert docker-compose.yml

**Docker Compose:**
```yaml
version: '3'
services:
  web:
    build: .
    ports:
      - "3000:3000"
    environment:
      - NODE_ENV=production
    depends_on:
      - db
      - redis
      
  worker:
    build: .
    command: npm run worker
    depends_on:
      - db
      - redis
      
  db:
    image: postgres:13
    environment:
      - POSTGRES_PASSWORD=secret
      
  redis:
    image: redis:alpine
```

**Convox convox.yml:**
```yaml
environment:
  - NODE_ENV=production

resources:
  database:
    type: postgres
  cache:
    type: redis

services:
  web:
    build: .
    port: 3000
    resources:
      - database
      - cache
      
  worker:
    build: .
    command: npm run worker
    resources:
      - database
      - cache
```

### Step 2: Deploy

```bash
# Create machine via Console first
$ convox cloud deploy -i production
```

## Common Migration Tasks

### Database Migration

#### PostgreSQL

```bash
# Export from source
$ pg_dump $OLD_DATABASE_URL > backup.sql

# Import to Convox
$ convox cloud resources import database --file backup.sql -a myapp -i production
```

#### MySQL

```bash
# Export from source
$ mysqldump -h old-host -u user -p database > backup.sql

# Import to Convox
$ convox cloud resources import database --file backup.sql -a myapp -i production
```

#### MongoDB (Using External Service)

Since Convox Cloud doesn't provide MongoDB:

```bash
# Use MongoDB Atlas or similar
$ convox cloud env set MONGODB_URI=mongodb+srv://... -a myapp -i production
```

### File Storage Migration

Convox Cloud doesn't support persistent volumes. Use object storage instead:

```bash
# Upload existing files to S3
$ aws s3 sync ./uploads s3://my-bucket/uploads

# Update app to use S3
$ convox cloud env set \
    S3_BUCKET=my-bucket \
    AWS_ACCESS_KEY_ID=... \
    AWS_SECRET_ACCESS_KEY=... \
    -a myapp -i production
```

### Scheduled Jobs Migration

**From Heroku Scheduler:**
```yaml
timers:
  cleanup:
    command: rake cleanup
    schedule: "0 3 * * *"
    service: web
```

**From cron:**
```yaml
timers:
  hourly:
    command: /app/bin/hourly-task
    schedule: "0 * * * *"
    service: worker
```

### SSL Certificates

Convox automatically provisions Let's Encrypt certificates:

```yaml
services:
  web:
    domain: myapp.com,www.myapp.com
    port: 3000
```

## Migration Checklist

### Pre-Migration

- [ ] Backup all data (database, files, configs)
- [ ] Document current environment variables
- [ ] Note current scaling configuration
- [ ] Identify external services and APIs
- [ ] Review application logs for issues
- [ ] Plan maintenance window

### During Migration

- [ ] Create Convox Cloud machine via Console
- [ ] Create Dockerfile if needed
- [ ] Create convox.yml
- [ ] Deploy application
- [ ] Import databases
- [ ] Set environment variables
- [ ] Configure domains
- [ ] Run smoke tests

### Post-Migration

- [ ] Verify all services are running
- [ ] Check application logs
- [ ] Test critical user paths
- [ ] Update DNS records
- [ ] Monitor for 24 hours
- [ ] Remove old infrastructure

## Troubleshooting Common Issues

### Build Failures

**Problem**: Build fails with "out of memory"

**Solution**: Optimize Dockerfile with multi-stage builds:
```dockerfile
# Build stage
FROM node:18 AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Runtime stage
FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
CMD ["node", "dist/index.js"]
```

### Port Configuration

**Problem**: Application not accessible

**Solution**: Ensure your app binds to `0.0.0.0`:
```javascript
// Wrong
app.listen(3000, 'localhost')

// Correct
app.listen(3000, '0.0.0.0')
```

### Database Connections

**Problem**: Can't connect to database

**Solution**: Use Convox-provided environment variables:
```javascript
// The DATABASE_URL is automatically set when you link a resource
const dbUrl = process.env.DATABASE_URL || 'postgres://localhost/dev'
```

### Memory Issues

**Problem**: Application runs out of memory

**Solution**: Adjust scaling configuration:
```yaml
services:
  web:
    scale:
      memory: 1024  # Increase from 512
```

## Getting Help

### Resources

- [Convox Documentation](https://docs.convox.com)
- [Community Forum](https://community.convox.com)
- [GitHub Examples](https://github.com/convox-examples)

### Support Channels

- Email: cloud-support@convox.com
- Community: community.convox.com
- Emergency: Contact support with your machine ID

## Next Steps

After migration:

1. [Optimize your machine size](/cloud/machines/sizing-and-pricing)
2. [Set up monitoring and alerts](/cloud/monitoring)
3. [Configure autoscaling](/deployment/scaling)
4. [Implement CI/CD](/deployment/workflows)