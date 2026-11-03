# Protocol: ps5-trophy

The trophy slice of the NP web API gateway `m.np.playstation.com`, path prefix `/api/trophy/v1/`. Shapes come from
the public `psn-api` client library (see `research/ps5/psn-endpoints.md` section 1.3); whether the PS5 system software
itself uses these paths is unverified. All requests need `Authorization: Bearer <OpenPak token>` (ps5-common `oauth`);
failures are `401` with the PSN error body. `{acct}` is an account id or `me` (the token's account).

| Request | Answer |
|---|---|
| `GET /users/{acct}/trophyTitles?limit&offset` | `{trophyTitles[], totalItemCount, nextOffset?, previousOffset?}`; each title: `npServiceName, npCommunicationId, trophySetVersion, trophyTitleName, trophyTitleDetail, trophyTitleIconUrl, trophyTitlePlatform, hasTrophyGroups, definedTrophies{bronze,silver,gold,platinum}, progress, earnedTrophies{...}, hiddenFlag, lastUpdatedDateTime`. Only titles with at least one earned trophy. |
| `GET /users/{acct}/trophySummary` | `{accountId, trophyLevel, progress, tier, earnedTrophies{...}}` |
| `GET /npCommunicationIds/{id}/trophyGroups` | title header + `trophyGroups[{trophyGroupId, trophyGroupName, trophyGroupDetail, trophyGroupIconUrl, definedTrophies}]` |
| `GET /npCommunicationIds/{id}/trophyGroups/{group}/trophies?limit&offset` | `{trophySetVersion, hasTrophyGroups, trophies[], totalItemCount, nextOffset?}`; `group` is a group id or `all` |
| `GET /users/{acct}/npCommunicationIds/{id}/trophyGroups` | `{trophySetVersion, hiddenFlag, progress, earnedTrophies, lastUpdatedDateTime, trophyGroups[{trophyGroupId, progress, earnedTrophies, lastUpdatedDateTime}]}` |
| `GET /users/{acct}/npCommunicationIds/{id}/trophyGroups/{group}/trophies` | the trophies list plus `earned` and `earnedDateTime` on each |
| anything else under `/api/trophy/` | logged at warn (method, path, query, UA), `404` PSN error body |

The `npServiceName` query parameter is accepted and ignored (the set's own service name, default `trophy2`, is used).
Trophy fields: `trophyId, trophyHidden, trophyType, trophyName, trophyDetail, trophyIconUrl, trophySmallIconUrl,
trophyRare, trophyEarnedRate` (+ `trophyProgressTargetValue`, `trophyRewardName`, `trophyRewardImageUrl` when set).
Defaults: `limit` 800 for titles, 200 for trophies.

## Storage and unlocks

Trophy-set definitions are JSON files (one set each) loaded from `PS5_TROPHY_SETS_DIR`: `npCommunicationId`,
`trophyTitleName`, `groups[{trophyGroupId, trophyGroupName, trophies[...]}]` (see `sets/example.json`). The group
id `default` is the base game; `all` is reserved. Earnings are stored in the achievements core behind the `store.Store` interface (`Achievements`); `Memory` is development only.

How the console reports an unlock is unknown. Until captured on hardware, `POST /internal/unlock` records one:

```
POST /internal/unlock        X-OpenPak-Internal-Key: <PS5_TROPHY_INTERNAL_KEY>
{"accountId":"42","npCommunicationId":"NPWR00000_00","trophyId":1}   ->  {"status":"ok"}
```

The first earn time wins; repeating is a no-op. Without the key configured the endpoint is `404`.

## Approximations (unverified)

- `progress` is the share of defined trophies earned (PSN weights by trophy type).
- Trophy level: points bronze 15, silver 30, gold 90, platinum 300; `trophyLevel = 1 + points/200` (max 999),
  `progress = points%200/2`, `tier = 1 + (level-1)/100`. Sony's real level table differs.
- `trophyRare` and `trophyEarnedRate` are served as stored in the set file; they are not computed from earnings.
