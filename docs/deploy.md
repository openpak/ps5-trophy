# Deploy: ps5-trophy

`m.np.playstation.com` is ONE host shared by ps5-profile, ps5-friends, ps5-trophy and ps5-messaging. DNS sends the
host to OpenPak; Traefik splits it by URL path. This service owns `/api/trophy/`.

```yaml
# Traefik dynamic configuration (file provider)
http:
  routers:
    ps5-trophy:
      rule: "Host(`m.np.playstation.com`) && PathPrefix(`/api/trophy/`)"
      entryPoints: [websecure]
      service: ps5-trophy
      tls: {}
      priority: 200          # longer than the host-only catch-all for the other m.np services
  services:
    ps5-trophy:
      loadBalancer:
        servers: [{url: "http://openpak-ps5-trophy:8080"}]
```

Do not strip the prefix: the service serves the full `/api/trophy/v1/...` path. The other services take
`/api/userProfile/`, `/api/gamingLoungeGroups/`, etc. on the same host with their own routers.

## Run

```sh
cp example.env .env   # set PS5_TOKEN_KEY (same key as ps5-auth)
set -a; source .env; set +a
PS5_TROPHY_SETS_DIR=sets GOWORK=off go run ./cmd/ps5-trophy
```

```sh
GOWORK=off go vet ./... && GOWORK=off go test ./...
docker build -t ps5-trophy .
```

## Configuration

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `PS5_TOKEN_KEY` | yes | | HMAC key of the bearer tokens (ps5-common `oauth`), at least 32 bytes |
| `PS5_ADDR` | no | `:8080` | HTTP listen address |
| `PS5_LOG_LEVEL` | no | `info` | accepted (ps5-common), logging is info-level |
| `PS5_TROPHY_SETS_DIR` | no | | directory of trophy-set `*.json` files, read at start |
| `PS5_TROPHY_INTERNAL_KEY` | no | | enables `POST /internal/unlock`; never route it through Traefik |

Other shared `PS5_*` variables are accepted and unused. Storage is in memory: earned trophies are lost on restart.

## Adding a title

Drop `<name>.json` (shape: `sets/example.json`) into the sets directory and restart. Rebuild nothing.

## Release

Tag `v*.*.*` publishes `ghcr.io/openpak/ps5-trophy`; deployed hosts auto-update from it.
