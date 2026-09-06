# Shared Component Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move all Mago, Frimodel, PostgreSQL, and Nginx deployment files to `/data/{mago,frimodel,component}`, expose PostgreSQL publicly on port `5432`, and keep every application and Redis port private on `friday_net`.

**Architecture:** `/data/mago` and `/data/frimodel` own product-specific Compose files, secrets, logs, and data. `/data/component` owns shared PostgreSQL and Nginx; `component-nginx` is the only web ingress, and `component-postgres` is the shared database endpoint for both product families.

**Tech Stack:** Docker Compose, Docker bridge networking, PostgreSQL 16, Redis 7.4, Nginx 1.27, rsync, SSH.

---

### Task 1: Capture current state and create destination hierarchy

**Files:**
- Create: `/data/mago/{api,web,studio,redis}`
- Create: `/data/frimodel/{master,worker,cpa,redis}`
- Create: `/data/component/{postgres,nginx}`

- [ ] **Step 1: Save a read-only live inventory in command output**

Run:

```bash
docker ps --format '{{.Names}}|{{.Image}}|{{.Status}}|{{.Ports}}' | sort
docker network inspect friday_net --format '{{range .Containers}}{{.Name}}|{{.IPv4Address}}{{println}}{{end}}' | sort
sudo ss -lntp
```

Expected: all current Mago services, both Redis containers, `friday-postgres`, and `mago-nginx` are running.

- [ ] **Step 2: Create the target directories**

Run:

```bash
sudo install -d -o ubuntu -g ubuntu -m 0750 \
  /data/mago/api \
  /data/mago/web \
  /data/mago/studio \
  /data/mago/redis/data \
  /data/frimodel/master \
  /data/frimodel/worker \
  /data/frimodel/cpa \
  /data/frimodel/redis/data \
  /data/frimodel/redis/secrets \
  /data/component/postgres/data \
  /data/component/postgres/backups \
  /data/component/postgres/secrets \
  /data/component/nginx/conf.d \
  /data/component/nginx/staged/frimodel \
  /data/component/nginx/certs
```

Expected: product and component service directories exist under `/data`.

### Task 2: Copy all live deployment files before downtime

**Files:**
- Copy: `/mago/mago-api/` to `/data/mago/api/`
- Copy: `/mago/mago-web/` to `/data/mago/web/`
- Copy: `/mago/mago-studio/` to `/data/mago/studio/`
- Copy: `/mago/mago-data/{docker-compose.yml,.env,redis_data}` to `/data/mago/redis/`
- Copy: live Master, Worker, CPA, and Friday Redis configuration into their matching `/data/frimodel/*` directories
- Copy: `/friday/postgres/` to `/data/component/postgres/`

- [ ] **Step 1: Copy product directories without deleting sources**

Run:

```bash
rsync -a /mago/mago-api/ /data/mago/api/
rsync -a /mago/mago-web/ /data/mago/web/
rsync -a /mago/mago-studio/ /data/mago/studio/
rsync -a /mago/mago-data/docker-compose.yml /mago/mago-data/.env /data/mago/redis/
rsync -a /mago/mago-data/redis_data/ /data/mago/redis/data/
rsync -a --exclude='data/' --exclude='logs/' /friday/frimodel/master/ /data/frimodel/master/
rsync -a --exclude='data/' --exclude='logs/' /friday/frimodel/worker/ /data/frimodel/worker/
rsync -a --exclude='pgstore/' --exclude='logs/' /friday/frimodel/cpa/ /data/frimodel/cpa/
rsync -a --exclude='data/' /friday/frimodel/redis/ /data/frimodel/redis/
rsync -a /friday/postgres/docker-compose.yml /data/component/postgres/
rsync -a /friday/postgres/secrets/ /data/component/postgres/secrets/
rsync -a /friday/postgres/backups/ /data/component/postgres/backups/
rsync -a /friday/postgres/data/ /data/component/postgres/data/
```

Expected: all required files exist in the target hierarchy while source services remain running.

- [ ] **Step 2: Split active and staged Nginx files**

Run:

```bash
rsync -a /mago/mago-web/nginx/conf.d/ /data/component/nginx/conf.d/
rsync -a /mago/mago-web/nginx/certs/ /data/component/nginx/certs/
rsync -a /friday/frimodel/nginx/conf.d/ /data/component/nginx/staged/frimodel/
```

Expected: Mago configuration is active-path material and Frimodel configuration remains staged.

### Task 3: Prepare target Compose and environment files offline

**Files:**
- Modify: `/data/component/postgres/docker-compose.yml`
- Create: `/data/component/nginx/docker-compose.yml`
- Modify: `/data/mago/api/docker-compose.yml`
- Modify: `/data/mago/web/docker-compose.yml`
- Modify: `/data/mago/studio/docker-compose.yml`
- Modify: `/data/mago/redis/docker-compose.yml`
- Modify: `/data/frimodel/{master,worker,cpa,redis}/docker-compose.yml`
- Modify: environment files containing `friday-postgres`

- [ ] **Step 1: Prepare shared PostgreSQL Compose**

Change the PostgreSQL container name to `component-postgres`, add:

```yaml
ports:
  - "0.0.0.0:5432:5432"
```

Keep relative mounts under `/data/component/postgres` and configure network aliases:

```yaml
networks:
  friday_net:
    aliases:
      - component-postgres
      - postgres
```

- [ ] **Step 2: Create independent shared Nginx Compose**

Create `/data/component/nginx/docker-compose.yml` with:

```yaml
services:
  nginx:
    image: nginx@sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10
    container_name: component-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./conf.d:/etc/nginx/conf.d:ro
      - ./certs:/etc/nginx/certs:ro
    networks:
      friday_net:
        aliases:
          - component-nginx
          - nginx
    logging:
      driver: json-file
      options:
        max-size: "100m"
        max-file: "5"

networks:
  friday_net:
    external: true
```

- [ ] **Step 3: Remove host application ports**

Replace the Mago API, Web, CMS, and Studio `ports` sections with internal `expose` entries:

```yaml
expose:
  - "3000"
```

Studio uses:

```yaml
expose:
  - "3010"
```

Remove the `nginx` service from `/data/mago/web/docker-compose.yml`.

- [ ] **Step 4: Normalize Redis mounts**

Set Mago Redis to:

```yaml
volumes:
  - /data/mago/redis/data:/data
```

Keep `mago-redis` persistent. Keep Friday Redis non-persistent and set its data mount to `/data/frimodel/redis/data`.

- [ ] **Step 5: Rewrite PostgreSQL Docker hostnames**

Replace `friday-postgres` with `component-postgres` in:

```text
/data/mago/api/.env
/data/mago/web/env/mago-cms.env
/data/mago/web/env/mago-web.env
/data/mago/studio/.env
/data/frimodel/master/.env
/data/frimodel/worker/.env
```

Keep credentials, database names, query parameters, and all non-host values unchanged.

- [ ] **Step 6: Validate target configuration before downtime**

Run `docker compose config --quiet` in every target Compose directory. Confirm only PostgreSQL has `published: 5432`, only Nginx has `published: 80/443`, and all other services have no published ports.

Expected: every target project validates without starting a container.

### Task 4: Offline-test shared Nginx before maintenance

**Files:**
- Use: `/data/component/nginx/conf.d/*.conf`
- Use: `/data/component/nginx/certs/*`
- Use: `/data/component/nginx/staged/frimodel/*.conf`

- [ ] **Step 1: Validate active Mago configuration**

Run:

```bash
docker run --rm --network friday_net \
  -v /data/component/nginx/conf.d:/etc/nginx/conf.d:ro \
  -v /data/component/nginx/certs:/etc/nginx/certs:ro \
  nginx:1.27-alpine nginx -t
```

Expected: syntax and configuration tests succeed.

- [ ] **Step 2: Validate Mago plus staged Frimodel configuration**

Copy active and staged `.conf` files into a temporary directory and run `nginx -t` with temporary host entries for `friday-new-api-master` and `friday-new-api-worker`.

Expected: the combined configuration validates; the temporary directory is deleted afterward.

### Task 5: Enter maintenance and run the final data sync

**Files:**
- Final-sync mutable application and database directories.

- [ ] **Step 1: Stop dependent applications first**

Run in order:

```bash
cd /mago/mago-web && docker compose down
cd /mago/mago-studio && docker compose down
cd /mago/mago-api && docker compose down
cd /mago/mago-data && docker compose down
cd /friday/frimodel/redis && docker compose down --remove-orphans
cd /friday/postgres && docker compose down
```

Expected: old Mago applications, both Redis projects, old Nginx, and old PostgreSQL stop cleanly.

- [ ] **Step 2: Run final rsync while services are stopped**

Run:

```bash
rsync -a --delete /mago/mago-api/apidata/ /data/mago/api/apidata/
rsync -a --delete /mago/mago-api/apilogs/ /data/mago/api/apilogs/
rsync -a --delete /mago/mago-data/redis_data/ /data/mago/redis/data/
rsync -a --delete /friday/postgres/data/ /data/component/postgres/data/
rsync -a --delete /friday/postgres/backups/ /data/component/postgres/backups/
```

Expected: target mutable directories exactly match stopped sources.

### Task 6: Start shared infrastructure and validate data

- [ ] **Step 1: Start `component-postgres`**

Run:

```bash
cd /data/component/postgres
docker compose up -d
```

Wait for health, then verify the expected databases and roles. Confirm host port `0.0.0.0:5432` is listening.

- [ ] **Step 2: Start both Redis projects**

Run:

```bash
cd /data/mago/redis && docker compose up -d
cd /data/frimodel/redis && docker compose up -d
```

Expected: `mago-redis` preserves AOF data; `friday-redis` is healthy, non-persistent, and returns `PONG` for DB 0 and DB 5.

### Task 7: Start Mago applications and shared Nginx

- [ ] **Step 1: Start Mago API, Web, and Studio**

Run:

```bash
cd /data/mago/api && docker compose up -d
cd /data/mago/web && docker compose up -d
cd /data/mago/studio && docker compose up -d
```

Wait for all configured health checks. Confirm none of these projects publishes host application ports.

- [ ] **Step 2: Start `component-nginx`**

Run:

```bash
cd /data/component/nginx
docker compose up -d
docker exec component-nginx nginx -t
```

Expected: Nginx binds `80/443`, Mago domains respond, and Frimodel server blocks remain staged.

### Task 8: Final audit and delete old paths

- [ ] **Step 1: Verify public and private ports**

Expected public listeners: SSH `22`, Nginx `80/443`, PostgreSQL `5432`. No Redis, CPA, New API, or Mago application port is published.

- [ ] **Step 2: Verify mounts, networks, health, domains, and database access**

Inspect all running containers, verify mounts point only into `/data`, confirm `friday_net`, and test current Mago domains through Nginx.

- [ ] **Step 3: Remove superseded paths**

After every check succeeds, delete only:

```text
/mago
/friday/postgres
/friday/frimodel
```

Do not delete `/friday` itself, `/data/docker`, or any unrelated server path.

- [ ] **Step 4: Produce final inventory**

Report container names, images, health, public ports, Docker aliases, mount sources, directory tree, database list, and Redis persistence modes.
