// Package api serves the trophy slice of m.np.playstation.com
// (/api/trophy/v1/...) over a store.Store.
package api

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/openpak/ps5-common/httpx"
	"github.com/openpak/ps5-common/oauth"
	"openpak/ps5-trophy/internal/store"
)

const prefix = "/api/trophy/v1"

// Server is the trophy API.
type Server struct {
	St          store.Store
	Verifier    oauth.Verifier
	InternalKey string // empty disables /internal/unlock
	Log         *slog.Logger
	Now         func() time.Time
}

// Handler returns the full handler (health, API, internal, catch-all).
func (s *Server) Handler() http.Handler {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Log == nil {
		s.Log = slog.Default()
	}
	m := httpx.Mux()
	api := http.NewServeMux()
	api.HandleFunc("GET "+prefix+"/users/{acct}/trophyTitles", s.userTitles)
	api.HandleFunc("GET "+prefix+"/users/{acct}/trophySummary", s.summary)
	api.HandleFunc("GET "+prefix+"/users/{acct}/npCommunicationIds/{id}/trophyGroups", s.userGroups)
	api.HandleFunc("GET "+prefix+"/users/{acct}/npCommunicationIds/{id}/trophyGroups/{group}/trophies", s.userTrophies)
	api.HandleFunc("GET "+prefix+"/npCommunicationIds/{id}/trophyGroups", s.groups)
	api.HandleFunc("GET "+prefix+"/npCommunicationIds/{id}/trophyGroups/{group}/trophies", s.trophies)
	// Unknown: how the console records an unlock. Log everything else under /api/trophy/.
	api.HandleFunc("/api/trophy/", s.catchAll)
	m.Handle("/api/trophy/", oauth.Require(s.Verifier)(api))
	m.HandleFunc("POST /internal/unlock", s.internalUnlock)
	return httpx.Logger(s.Log, m)
}

func (s *Server) catchAll(w http.ResponseWriter, r *http.Request) {
	s.Log.Warn("unhandled trophy request", "method", r.Method, "path", r.URL.Path, "query", r.URL.RawQuery, "ua", r.UserAgent())
	httpx.WriteError(w, http.StatusNotFound, 2240526, "Resource not found")
}

// account resolves {acct}: "me" is the token's account.
func account(r *http.Request) string {
	a := r.PathValue("acct")
	if a == "me" {
		if c, ok := oauth.FromContext(r.Context()); ok {
			return c.AccountID
		}
	}
	return a
}

func page(r *http.Request, def int) (limit, offset int) {
	limit, offset = def, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = v
	}
	return
}

// window slices [0,n) by limit/offset and returns the bounds plus paging fields.
func window(n, limit, offset int) (lo, hi int, next, prev *int) {
	lo = min(offset, n)
	hi = min(lo+limit, n)
	if hi < n {
		v := hi
		next = &v
	}
	if lo > 0 {
		v := max(lo-limit, 0)
		prev = &v
	}
	return
}

type counts struct {
	Bronze   int `json:"bronze"`
	Silver   int `json:"silver"`
	Gold     int `json:"gold"`
	Platinum int `json:"platinum"`
}

func (c *counts) add(typ string) {
	switch typ {
	case store.Bronze:
		c.Bronze++
	case store.Silver:
		c.Silver++
	case store.Gold:
		c.Gold++
	case store.Platinum:
		c.Platinum++
	}
}

func (c counts) points() int { return c.Bronze*15 + c.Silver*30 + c.Gold*90 + c.Platinum*300 }

func ts(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

func hasGroups(set store.Set) bool { return len(set.Groups) > 1 }

// tally counts defined trophies and earned trophies, optionally for one group ("" or "all" = all).
func tally(set store.Set, earned map[int]store.Earned, group string) (def, got counts, nDef, nGot int, last time.Time) {
	for _, g := range set.Groups {
		if group != "" && group != "all" && g.ID != group {
			continue
		}
		for _, t := range g.Trophies {
			def.add(t.Type)
			nDef++
			if e, ok := earned[t.ID]; ok {
				got.add(t.Type)
				nGot++
				if e.At.After(last) {
					last = e.At
				}
			}
		}
	}
	return
}

func progress(nDef, nGot int) int {
	if nDef == 0 {
		return 0
	}
	return nGot * 100 / nDef
}

func (s *Server) userTitles(w http.ResponseWriter, r *http.Request) {
	acct := account(r)
	limit, offset := page(r, 800)
	ids := s.St.Played(acct)
	lo, hi, next, prev := window(len(ids), limit, offset)
	titles := []map[string]any{}
	for _, id := range ids[lo:hi] {
		set, ok := s.St.Set(id)
		if !ok {
			continue
		}
		earned := s.St.Earnings(acct, id)
		def, got, nDef, nGot, last := tally(set, earned, "")
		titles = append(titles, map[string]any{
			"npServiceName": set.NpServiceName, "npCommunicationId": set.NpCommunicationID,
			"trophySetVersion": set.Version, "trophyTitleName": set.Name, "trophyTitleDetail": set.Detail,
			"trophyTitleIconUrl": set.IconURL, "trophyTitlePlatform": set.Platform,
			"hasTrophyGroups": hasGroups(set), "definedTrophies": def, "progress": progress(nDef, nGot),
			"earnedTrophies": got, "hiddenFlag": false, "lastUpdatedDateTime": ts(last),
		})
	}
	out := map[string]any{"trophyTitles": titles, "totalItemCount": len(ids)}
	if next != nil {
		out["nextOffset"] = *next
	}
	if prev != nil {
		out["previousOffset"] = *prev
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// summary: level and tier are an approximation (see docs/protocol.md).
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	acct := account(r)
	var total counts
	for _, id := range s.St.Played(acct) {
		set, ok := s.St.Set(id)
		if !ok {
			continue
		}
		_, got, _, _, _ := tally(set, s.St.Earnings(acct, id), "")
		total.Bronze += got.Bronze
		total.Silver += got.Silver
		total.Gold += got.Gold
		total.Platinum += got.Platinum
	}
	pts := total.points()
	level := min(1+pts/200, 999)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"accountId": acct, "trophyLevel": level, "progress": pts % 200 / 2,
		"tier": min(1+(level-1)/100, 10), "earnedTrophies": total,
	})
}

func (s *Server) set(w http.ResponseWriter, r *http.Request) (store.Set, bool) {
	set, ok := s.St.Set(r.PathValue("id"))
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, 2240526, "Resource not found")
	}
	return set, ok
}

func (s *Server) groups(w http.ResponseWriter, r *http.Request) {
	set, ok := s.set(w, r)
	if !ok {
		return
	}
	def, _, _, _, _ := tally(set, nil, "")
	gs := []map[string]any{}
	for _, g := range set.Groups {
		gdef, _, _, _, _ := tally(set, nil, g.ID)
		gs = append(gs, map[string]any{"trophyGroupId": g.ID, "trophyGroupName": g.Name, "trophyGroupDetail": g.Detail,
			"trophyGroupIconUrl": g.IconURL, "definedTrophies": gdef})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"trophySetVersion": set.Version, "trophyTitleName": set.Name, "trophyTitleDetail": set.Detail,
		"trophyTitleIconUrl": set.IconURL, "trophyTitlePlatform": set.Platform,
		"hasTrophyGroups": hasGroups(set), "definedTrophies": def, "trophyGroups": gs,
	})
}

func (s *Server) userGroups(w http.ResponseWriter, r *http.Request) {
	set, ok := s.set(w, r)
	if !ok {
		return
	}
	earned := s.St.Earnings(account(r), set.NpCommunicationID)
	def, got, nDef, nGot, last := tally(set, earned, "")
	gs := []map[string]any{}
	for _, g := range set.Groups {
		gd, gg, gnD, gnG, gl := tally(set, earned, g.ID)
		_ = gd
		gs = append(gs, map[string]any{"trophyGroupId": g.ID, "progress": progress(gnD, gnG),
			"earnedTrophies": gg, "lastUpdatedDateTime": ts(gl)})
	}
	_ = def
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"trophySetVersion": set.Version, "hiddenFlag": false, "progress": progress(nDef, nGot),
		"earnedTrophies": got, "lastUpdatedDateTime": ts(last), "trophyGroups": gs,
	})
}

// groupTrophies returns the trophies of group ("all" = every group).
func groupTrophies(set store.Set, group string) ([]store.Trophy, bool) {
	var out []store.Trophy
	found := group == "all"
	for _, g := range set.Groups {
		if group == "all" || g.ID == group {
			found = true
			out = append(out, g.Trophies...)
		}
	}
	return out, found
}

func (s *Server) trophies(w http.ResponseWriter, r *http.Request) { s.trophyList(w, r, false) }

func (s *Server) userTrophies(w http.ResponseWriter, r *http.Request) { s.trophyList(w, r, true) }

func (s *Server) trophyList(w http.ResponseWriter, r *http.Request, user bool) {
	set, ok := s.set(w, r)
	if !ok {
		return
	}
	list, found := groupTrophies(set, r.PathValue("group"))
	if !found {
		httpx.WriteError(w, http.StatusNotFound, 2240526, "Resource not found")
		return
	}
	limit, offset := page(r, 200)
	lo, hi, next, prev := window(len(list), limit, offset)
	var earned map[int]store.Earned
	if user {
		earned = s.St.Earnings(account(r), set.NpCommunicationID)
	}
	rows := make([]map[string]any, 0, hi-lo)
	for _, t := range list[lo:hi] {
		row := map[string]any{
			"trophyId": t.ID, "trophyHidden": t.Hidden, "trophyType": t.Type, "trophyName": t.Name,
			"trophyDetail": t.Detail, "trophyIconUrl": t.IconURL, "trophySmallIconUrl": t.SmallIconURL,
			"trophyRare": t.Rare, "trophyEarnedRate": t.EarnedRate,
		}
		if t.ProgressTargetValue != "" {
			row["trophyProgressTargetValue"] = t.ProgressTargetValue
		}
		if t.RewardName != "" {
			row["trophyRewardName"] = t.RewardName
			row["trophyRewardImageUrl"] = t.RewardImageURL
		}
		if user {
			e, got := earned[t.ID]
			row["earned"] = got
			if got {
				row["earnedDateTime"] = ts(e.At)
			}
		}
		rows = append(rows, row)
	}
	out := map[string]any{"trophySetVersion": set.Version, "hasTrophyGroups": hasGroups(set), "trophies": rows, "totalItemCount": len(list)}
	if next != nil {
		out["nextOffset"] = *next
	}
	if prev != nil {
		out["previousOffset"] = *prev
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// internalUnlock is OpenPak-internal (tests, tooling): the console's real unlock call is unknown.
func (s *Server) internalUnlock(w http.ResponseWriter, r *http.Request) {
	if s.InternalKey == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-OpenPak-Internal-Key")), []byte(s.InternalKey)) != 1 {
		httpx.WriteError(w, http.StatusNotFound, 2240526, "Resource not found")
		return
	}
	var req struct {
		AccountID         string `json:"accountId"`
		NpCommunicationID string `json:"npCommunicationId"`
		TrophyID          int    `json:"trophyId"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil || req.AccountID == "" {
		httpx.WriteError(w, http.StatusBadRequest, 2240001, "Bad request")
		return
	}
	if err := s.St.Unlock(req.AccountID, req.NpCommunicationID, req.TrophyID, s.Now()); err != nil {
		if errors.Is(err, store.ErrNoSet) || errors.Is(err, store.ErrNoTrophy) {
			httpx.WriteError(w, http.StatusNotFound, 2240526, err.Error())
		} else {
			httpx.WriteError(w, http.StatusBadGateway, 2240001, "Trophy storage unavailable")
		}
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
