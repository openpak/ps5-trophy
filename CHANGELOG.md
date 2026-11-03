# Changelog: ps5-trophy

## v0.1.0 (2026-11-03)

- 2026-11-03: earned trophies are stored in the achievements core (`store.Achievements`, network `np`, like np-trophy); set
  definitions still load from JSON. `PS5_ACHIEVEMENTS_ADDR` / `PS5_ACHIEVEMENTS_KEY` are required unless
  `PS5_DEV_MEMORY=1`. `/internal/unlock` answers 502 when the core is down. Restart test over a fake core. Not deployed.

- 2026-11-02: first cut. The `/api/trophy/v1/` slice of m.np.playstation.com (trophyTitles, trophySummary,
  trophyGroups, trophies, per-account earned variants) from the psn-api shapes; trophy sets load from JSON files;
  in-memory earnings behind a store interface; `POST /internal/unlock` for tests; logged 404 catch-all under
  `/api/trophy/`; ps5-common oauth `Require`. Not deployed, no tag; the console's real unlock call is unknown.
