# Shared Component Layout Design

## Goal

Move all Mago, Frimodel, and shared infrastructure deployment directories on `148.113.178.75` to the largest mounted disk, `/data`, while preserving current application data and reducing public host ports to SSH, Nginx, and PostgreSQL.

## Target layout

```text
/data/
├── mago/
│   ├── api/
│   ├── web/
│   ├── studio/
│   └── redis/
├── frimodel/
│   ├── master/
│   ├── worker/
│   ├── cpa/
│   └── redis/
└── component/
    ├── postgres/
    │   ├── data/
    │   ├── backups/
    │   └── secrets/
    └── nginx/
        ├── conf.d/
        ├── staged/frimodel/
        └── certs/
```

No symlinks remain after migration. The old `/mago`, `/friday/postgres`, and `/friday/frimodel` trees are removed only after full validation.

## Container names

| Responsibility | Container name |
| --- | --- |
| Shared PostgreSQL | `component-postgres` |
| Shared Nginx | `component-nginx` |
| Mago Redis | `mago-redis` |
| Frimodel Redis | `friday-redis` |

Existing application container names remain unchanged.

The PostgreSQL Docker aliases are `component-postgres` and `postgres`. All Mago and Frimodel environment files use `component-postgres:5432` after migration.

## Public ports

| Service | Host binding |
| --- | --- |
| SSH | Existing port `22` |
| Shared Nginx | `80:80`, `443:443` |
| Shared PostgreSQL | `0.0.0.0:5432:5432` |
| All Redis and application services | None |

Mago API, Web, CMS, and Studio lose their current loopback-only host mappings. Nginx continues to reach them through Docker DNS on `friday_net`.

The PostgreSQL public binding is explicitly requested. Host authentication remains SCRAM-SHA-256, and the existing strong PostgreSQL credentials remain unchanged.

## Redis isolation

The two Redis instances remain separate:

- `mago-redis` keeps AOF persistence and its existing Mago credentials and data.
- `friday-redis` remains non-persistent with `256mb`, `allkeys-lru`, and 16 logical databases. New API uses DB 0 and CPA uses DB 5.
- Neither Redis publishes port `6379` on the host.

## PostgreSQL migration

The existing shared PostgreSQL data directory moves from `/friday/postgres/data` to `/data/component/postgres/data`. Backups and secrets move under the same component directory.

PostgreSQL is stopped before the final data sync. The new Compose project starts the same image and data directory under the `component-postgres` container name. Existing databases and roles are preserved.

The Mago and Frimodel environment files are rewritten from `friday-postgres` to `component-postgres` before application restart.

## Nginx migration

Nginx is removed from the Mago Web Compose project and becomes an independent shared Compose project under `/data/component/nginx`.

The active Mago configuration and certificates move into the shared component directory. Frimodel configuration remains staged under `/data/component/nginx/staged/frimodel` until New API Master and Worker are running and healthy.

The Nginx container remains attached to `friday_net` and continues to proxy by container DNS. Only one Nginx container binds host ports `80/443`.

## Migration sequence

1. Record the current container, image, network, mount, and health state.
2. Create the complete `/data` target hierarchy.
3. Copy all deployment configuration and mutable data while services remain online.
4. Rewrite Compose files and environment hostnames in the copied target hierarchy.
5. Validate every target Compose project and offline Nginx configuration.
6. Stop Mago application containers, shared Nginx, both Redis containers, and PostgreSQL.
7. Run a final rsync for PostgreSQL, Redis, logs, and application data.
8. Start `component-postgres`, `mago-redis`, and `friday-redis`.
9. Validate PostgreSQL databases, Redis connectivity, aliases, and public port `5432`.
10. Start Mago API, Web, CMS, Adapter, Studio, and Studio Worker without host application ports.
11. Start `component-nginx`, validate domains, and verify Mago health.
12. Keep Frimodel Master, Worker, CPA, and staged Nginx server blocks inactive.
13. Remove old deployment paths only after all health and data checks pass.

## Validation

- Every Compose project uses the external `friday_net` network.
- Only SSH, Nginx `80/443`, and PostgreSQL `5432` listen publicly.
- Mago API, Web, CMS, Adapter, Studio, and both Redis containers have no published host ports.
- `component-postgres` contains the existing Mago databases and the prepared `gateway` database.
- Mago Redis preserves its data and returns `PONG` with its existing password.
- Friday Redis returns `PONG` for DB 0 and DB 5 and remains non-persistent.
- Current Mago domains return their expected status codes through `component-nginx`.
- Active Nginx configuration passes `nginx -t`.
- Staged Frimodel configuration passes an offline combined Nginx test.
- Old directories are absent only after the new layout is confirmed healthy.

## Failure handling

The copied target directories are fully validated before the maintenance window. If PostgreSQL, Redis, an application, or Nginx fails after the stop point, the migration stops immediately and the failing component is diagnosed before dependent services start. Unrelated old paths are not deleted until all target services pass validation.
