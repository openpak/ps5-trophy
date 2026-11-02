# Changelog: ps5-trophy

## Unreleased

- 2026-11-02: first cut. The `/api/trophy/v1/` slice of m.np.playstation.com (trophyTitles, trophySummary,
  trophyGroups, trophies, per-account earned variants) from the psn-api shapes; trophy sets load from JSON files;
  in-memory earnings behind a store interface; `POST /internal/unlock` for tests; logged 404 catch-all under
  `/api/trophy/`; ps5-common oauth `Require`. Not deployed, no tag; the console's real unlock call is unknown.
