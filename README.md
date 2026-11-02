# PS5 Trophy Server

OpenPak's PS5 Trophy Server.

| | |
| --- | --- |
| Network | PlayStation 5 |
| Role | The trophy slice (`/api/trophy/v1/`) of the `m.np.playstation.com` gateway: trophy titles, groups, trophies and summary |
| Depends on | [`ps5-common`](https://github.com/openpak/ps5-common) (vendored), bearer tokens from `ps5-auth` |
| Image | `ghcr.io/openpak/ps5-trophy` |
| Licence | AGPL-3.0-only |

## What it serves

| Endpoint | What |
| --- | --- |
| `GET /api/trophy/v1/users/{acct}/trophyTitles` | Titles the account has earned trophies in |
| `GET /api/trophy/v1/users/{acct}/trophySummary` | Level, progress, tier, earned counts |
| `GET /api/trophy/v1/npCommunicationIds/{id}/trophyGroups` | Groups of a trophy set |
| `GET /api/trophy/v1/npCommunicationIds/{id}/trophyGroups/{group}/trophies` | Trophies of a group (`all` = every group) |
| `GET /api/trophy/v1/users/{acct}/npCommunicationIds/{id}/trophyGroups[/{group}/trophies]` | Per-account progress and earned flags |
| `POST /internal/unlock` | OpenPak-internal unlock recording (key-guarded, never routed publicly) |
| `/healthz` | Health check |

## Run

```sh
PS5_TOKEN_KEY=<32+ bytes> PS5_TROPHY_SETS_DIR=sets GOWORK=off go run ./cmd/ps5-trophy
```

## Docs

- [Protocol](docs/protocol.md)
- [Deploy](docs/deploy.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
