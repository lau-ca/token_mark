# Frimodel Server Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate Frimodel configuration under `/friday/frimodel`, place mutable runtime data under `/data/frimodel`, and keep all Frimodel services private on `friday_net` except the existing Mago Nginx ports `80/443`.

**Architecture:** The existing `mago-nginx` remains the only public ingress. Master, Worker, CPA, Redis, and PostgreSQL communicate by Docker DNS on the external `friday_net`; configuration and secrets live on the system deployment disk while mutable data and logs live on the larger `/data` disk.

**Tech Stack:** Docker Compose, Docker bridge networking, Nginx, PostgreSQL 16, Redis 7.4, rsync, SSH.

---

### Task 1: Create the structured destination directories

**Files:**
- Create: `/friday/frimodel/master/`
- Create: `/friday/frimodel/worker/`
- Create: `/friday/frimodel/cpa/`
- Create: `/friday/frimodel/redis/secrets/`
- Create: `/friday/frimodel/nginx/conf.d/`
- Create: `/data/frimodel/{master,worker,cpa,redis}/`

- [ ] **Step 1: Capture the current Frimodel and Mago state**

Run:

```bash
ssh ubuntu@148.113.178.75 '
docker ps --format "{{.Names}}|{{.Status}}|{{.Ports}}" | sort
sudo ss -lntp
'
```

Expected: Mago containers and `friday-postgres` are running; only existing Nginx publishes `80/443`; Frimodel Master, Worker, and CPA are not running.

- [ ] **Step 2: Create configuration and runtime directories**

Run:

```bash
ssh ubuntu@148.113.178.75 '
sudo install -d -o ubuntu -g ubuntu -m 0750 \
  /friday/frimodel/master \
  /friday/frimodel/worker \
  /friday/frimodel/cpa \
  /friday/frimodel/redis/secrets \
  /friday/frimodel/nginx/conf.d \
  /data/frimodel/master/data \
  /data/frimodel/master/logs \
  /data/frimodel/worker/data \
  /data/frimodel/worker/logs \
  /data/frimodel/cpa/pgstore \
  /data/frimodel/cpa/logs \
  /data/frimodel/redis/data
'
```

Expected: all directories exist and are owned by `ubuntu:ubuntu`.

### Task 2: Copy current configuration and runtime data into the new layout

**Files:**
- Copy: `/data/services/frimodel-gateway/master/{.env,docker-compose.yml}`
- Copy: `/data/services/frimodel-gateway/worker/{.env,docker-compose.yml}`
- Copy: `/data/services/frimodel-gateway/cpa/{.env,config.yaml,docker-compose.yml}`
- Copy: `/data/services/frimodel-gateway/nginx/integration/*.conf`
- Copy: `/friday/redis/{docker-compose.yml,secrets/redis_password}`

- [ ] **Step 1: Copy configuration without deleting the source**

Run on the destination server:

```bash
rsync -a /data/services/frimodel-gateway/master/.env /data/services/frimodel-gateway/master/docker-compose.yml /friday/frimodel/master/
rsync -a /data/services/frimodel-gateway/worker/.env /data/services/frimodel-gateway/worker/docker-compose.yml /friday/frimodel/worker/
rsync -a /data/services/frimodel-gateway/cpa/.env /data/services/frimodel-gateway/cpa/config.yaml /data/services/frimodel-gateway/cpa/docker-compose.yml /friday/frimodel/cpa/
rsync -a /data/services/frimodel-gateway/nginx/integration/ /friday/frimodel/nginx/conf.d/
rsync -a /friday/redis/docker-compose.yml /friday/frimodel/redis/
rsync -a /friday/redis/secrets/redis_password /friday/frimodel/redis/secrets/
```

Expected: destination configuration files match their sources before path rewrites.

- [ ] **Step 2: Copy mutable data directories**

Run:

```bash
rsync -a /data/services/frimodel-gateway/master/data/ /data/frimodel/master/data/
rsync -a /data/services/frimodel-gateway/master/logs/ /data/frimodel/master/logs/
rsync -a /data/services/frimodel-gateway/worker/data/ /data/frimodel/worker/data/
rsync -a /data/services/frimodel-gateway/worker/logs/ /data/frimodel/worker/logs/
rsync -a /data/services/frimodel-gateway/cpa/pgstore/ /data/frimodel/cpa/pgstore/
rsync -a /data/services/frimodel-gateway/cpa/logs/ /data/frimodel/cpa/logs/
rsync -a /friday/redis/data/ /data/frimodel/redis/data/
```

Expected: data is copied before any old path is removed.

### Task 3: Rewrite Compose bind mounts and validate private networking

**Files:**
- Modify: `/friday/frimodel/master/docker-compose.yml`
- Modify: `/friday/frimodel/worker/docker-compose.yml`
- Modify: `/friday/frimodel/cpa/docker-compose.yml`
- Modify: `/friday/frimodel/redis/docker-compose.yml`

- [ ] **Step 1: Rewrite runtime mounts**

Apply these exact replacements:

```text
/data/services/frimodel-gateway/master/data -> /data/frimodel/master/data
/data/services/frimodel-gateway/master/logs -> /data/frimodel/master/logs
/data/services/frimodel-gateway/worker/data -> /data/frimodel/worker/data
/data/services/frimodel-gateway/worker/logs -> /data/frimodel/worker/logs
./pgstore -> /data/frimodel/cpa/pgstore
./logs -> /data/frimodel/cpa/logs
./data:/data -> /data/frimodel/redis/data:/data
```

Keep `.env`, `config.yaml`, and Redis secret references relative to their new configuration directories.

- [ ] **Step 2: Validate every Compose project**

Run:

```bash
for dir in \
  /friday/frimodel/master \
  /friday/frimodel/worker \
  /friday/frimodel/cpa \
  /friday/frimodel/redis; do
  cd "$dir"
  docker compose config --quiet
  docker compose config --networks
done
```

Expected: all configurations validate and every project reports only `friday_net`.

- [ ] **Step 3: Confirm no Frimodel host ports are published**

Run:

```bash
for dir in /friday/frimodel/master /friday/frimodel/worker /friday/frimodel/cpa /friday/frimodel/redis; do
  cd "$dir"
  docker compose config | grep -n "published:" && exit 1 || true
done
```

Expected: no `published:` entries.

### Task 4: Recreate only Frimodel Redis from the new directory

**Files:**
- Use: `/friday/frimodel/redis/docker-compose.yml`
- Use: `/friday/frimodel/redis/secrets/redis_password`

- [ ] **Step 1: Stop the old Frimodel Redis Compose project**

Run:

```bash
cd /friday/redis
docker compose down --remove-orphans
```

Expected: only `friday-redis` stops and is removed; `mago-redis` remains healthy.

- [ ] **Step 2: Start Redis from the new configuration directory**

Run:

```bash
cd /friday/frimodel/redis
docker compose up -d
```

Expected: `friday-redis` starts on `friday_net` without a published port.

- [ ] **Step 3: Verify Redis DB 0 and DB 5**

Run:

```bash
password=$(tr -d '\n' < /friday/frimodel/redis/secrets/redis_password)
docker run --rm --network friday_net redis:7.4-alpine redis-cli -h friday-redis -a "$password" --no-auth-warning -n 0 ping
docker run --rm --network friday_net redis:7.4-alpine redis-cli -h friday-redis -a "$password" --no-auth-warning -n 5 ping
```

Expected: both commands return `PONG`.

### Task 5: Validate staged Nginx integration and existing services

**Files:**
- Use: `/friday/frimodel/nginx/conf.d/*.conf`
- Preserve: `/mago/mago-web/nginx/conf.d/*.conf`

- [ ] **Step 1: Run an offline combined Nginx syntax test**

Run:

```bash
validation=/tmp/frimodel-nginx-validation
install -d -m 0750 "$validation"
cp /mago/mago-web/nginx/conf.d/mago.conf "$validation/mago.conf"
cp /mago/mago-web/nginx/conf.d/magos.conf "$validation/magos.conf"
cp /friday/frimodel/nginx/conf.d/*.conf "$validation/"
docker run --rm \
  --network friday_net \
  --add-host friday-new-api-master:127.0.0.1 \
  --add-host friday-new-api-worker:127.0.0.1 \
  -v /tmp/frimodel-nginx-validation:/etc/nginx/conf.d:ro \
  -v /mago/mago-web/nginx/certs:/etc/nginx/certs:ro \
  nginx:1.27-alpine nginx -t
find "$validation" -type f -delete
rmdir "$validation"
```

Expected: `syntax is ok` and `test is successful`.

- [ ] **Step 2: Confirm active Mago health and port ownership**

Run:

```bash
docker ps --format "{{.Names}}|{{.Status}}|{{.Ports}}" | sort
docker exec mago-nginx nginx -t
sudo ss -lntp | grep -E ':(80|443|5432|6379|8317)[[:space:]]'
```

Expected: existing Mago services remain healthy; only Nginx publishes `80/443`; no host listeners exist for PostgreSQL, Redis, or CPA.

### Task 6: Remove superseded paths after validation

**Files:**
- Remove: `/data/services/frimodel-gateway/`
- Remove: `/friday/redis/`

- [ ] **Step 1: Compare destination files and verify required secrets**

Run:

```bash
test -s /friday/frimodel/master/.env
test -s /friday/frimodel/worker/.env
test -s /friday/frimodel/cpa/.env
test -s /friday/frimodel/cpa/config.yaml
test -s /friday/frimodel/redis/secrets/redis_password
stat -c '%a|%U:%G|%n' \
  /friday/frimodel/master/.env \
  /friday/frimodel/worker/.env \
  /friday/frimodel/cpa/.env \
  /friday/frimodel/cpa/config.yaml \
  /friday/frimodel/redis/secrets/redis_password
```

Expected: all required files exist under `/friday/frimodel` and every secret reports mode `600`.

- [ ] **Step 2: Delete only superseded Frimodel paths**

Run:

```bash
find /data/services/frimodel-gateway -depth -delete
find /friday/redis -depth -delete
```

Expected: both superseded paths are absent. Do not touch `/friday/postgres`, `/mago`, `/data/docker`, or active Mago configuration.

- [ ] **Step 3: Produce the final inventory**

Run:

```bash
find /friday/frimodel /data/frimodel -maxdepth 4 -printf '%y|%M|%u:%g|%p\n' | sort
docker inspect friday-redis --format '{{.Name}}|{{.State.Health.Status}}|{{.HostConfig.NetworkMode}}|{{json .NetworkSettings.Ports}}'
```

Expected: only the new structured directories remain and `friday-redis` is healthy with no host port mapping.
