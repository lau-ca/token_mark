# Frimodel Server Layout Design

## Goal

Prepare the Frimodel gateway services on `148.113.178.75` without changing the running Mago traffic path. Keep only the existing Nginx ports `80/443` public and use the external Docker network `friday_net` for all Frimodel service-to-service traffic.

## Service network

| Service | Internal address | Host port |
| --- | --- | --- |
| New API master | `friday-new-api-master:3000` | None |
| New API worker | `friday-new-api-worker:3000` | None |
| CPA | `friday-cpa:8317` | None |
| Redis | `friday-redis:6379` | None |
| PostgreSQL | `friday-postgres:5432` | None |
| Existing Mago Nginx | N/A | `80/443` |

The existing Mago Nginx remains the only public ingress. Frimodel must not run a second host-networked Nginx container.

## Nginx routing

- `api.frimodel.com` proxies API and image relay routes to `friday-new-api-worker:3000`.
- `platform.frimodel.com` proxies the management UI to `friday-new-api-master:3000`.
- Platform routes that intentionally use worker relay behavior continue to proxy to the shared `new_api_workers` upstream.
- Frimodel server blocks remain staged until the database migration and Frimodel containers are healthy.

## Directory layout

Configuration and secrets use the system deployment disk:

```text
/friday/frimodel/
├── master/
│   ├── docker-compose.yml
│   └── .env
├── worker/
│   ├── docker-compose.yml
│   └── .env
├── cpa/
│   ├── docker-compose.yml
│   ├── .env
│   └── config.yaml
├── redis/
│   ├── docker-compose.yml
│   └── secrets/
└── nginx/
    └── conf.d/
```

Mutable runtime data uses the larger `/data` disk:

```text
/data/frimodel/
├── master/
│   ├── data/
│   └── logs/
├── worker/
│   ├── data/
│   └── logs/
├── cpa/
│   ├── pgstore/
│   └── logs/
└── redis/
    └── data/
```

The shared PostgreSQL deployment remains at `/friday/postgres` and is not moved.

## Migration behavior

1. Stop only the new `friday-redis` container.
2. Move its Compose configuration and secret to `/friday/frimodel/redis` and its empty runtime directory to `/data/frimodel/redis/data`.
3. Move the prepared Master, Worker, CPA, and Nginx configuration into `/friday/frimodel`.
4. Move empty or prepared runtime directories into `/data/frimodel`.
5. Rewrite bind mounts to the new runtime paths.
6. Recreate only `friday-redis` and verify DB 0 and DB 5.
7. Keep New API, CPA, and Frimodel Nginx server blocks inactive.

Existing Mago containers, `friday-postgres`, and active Nginx configuration must not be restarted or reloaded.

## Validation

- Run `docker compose config --quiet` for Master, Worker, CPA, and Redis.
- Confirm all four Compose projects use `friday_net`.
- Confirm Frimodel Compose files contain no published host ports.
- Confirm the host has no listeners on `5432`, `6379`, or `8317`.
- Confirm `friday-redis` is healthy and accepts authenticated connections to DB 0 and DB 5.
- Run an offline Nginx configuration test combining current Mago configuration with staged Frimodel configuration.
- Confirm all existing Mago containers remain in their prior running or healthy state.
- Remove the superseded `/data/services/frimodel-gateway` and `/friday/redis` directories only after their replacements validate successfully.

## Failure handling

If Redis recreation or configuration validation fails, keep the new structured directories for inspection and do not modify Mago services, PostgreSQL, DNS, or active Nginx configuration. Because New API and CPA are not running, a Redis-only retry does not affect current production traffic.
