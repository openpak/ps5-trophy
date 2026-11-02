package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openpak/ps5-common/oauth"
	"openpak/ps5-trophy/internal/store"
)

const setJSON = `{"npCommunicationId":"NPWR99999_00","trophyTitleName":"Test Game","groups":[
 {"trophyGroupId":"default","trophyGroupName":"Test Game","trophies":[
  {"trophyId":0,"trophyType":"platinum","trophyName":"All","trophyDetail":"d"},
  {"trophyId":1,"trophyType":"bronze","trophyName":"One","trophyDetail":"d"}]},
 {"trophyGroupId":"001","trophyGroupName":"DLC","trophies":[
  {"trophyId":2,"trophyType":"gold","trophyName":"Two","trophyDetail":"d"}]}]}`

func setup(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(setJSON), 0o600)
	st := store.NewMemory()
	if n, err := store.LoadDir(st, dir); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	sg, _ := oauth.NewSigner([]byte("0123456789abcdef0123456789abcdef"))
	tok, _ := sg.Sign(oauth.Claims{AccountID: "42", OnlineID: "tester", Expires: time.Now().Add(time.Hour).Unix()})
	s := &Server{St: st, Verifier: sg, InternalKey: "k"}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv, tok
}

func get(t *testing.T, srv *httptest.Server, tok, path string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("GET", srv.URL+path, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	json.Unmarshal(b, &m)
	return resp.StatusCode, m
}

func unlock(t *testing.T, srv *httptest.Server, key, body string) int {
	req, _ := http.NewRequest("POST", srv.URL+"/internal/unlock", strings.NewReader(body))
	req.Header.Set("X-OpenPak-Internal-Key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestFlow(t *testing.T) {
	srv, tok := setup(t)
	if c, _ := get(t, srv, "", "/api/trophy/v1/users/me/trophyTitles"); c != 401 {
		t.Fatalf("unauthenticated: %d", c)
	}
	_, m := get(t, srv, tok, "/api/trophy/v1/users/me/trophyTitles")
	if m["totalItemCount"].(float64) != 0 {
		t.Fatalf("titles before unlock: %v", m)
	}
	if c := unlock(t, srv, "wrong", `{}`); c != 404 {
		t.Fatalf("bad key: %d", c)
	}
	for _, id := range []string{"1", "2"} {
		if c := unlock(t, srv, "k", `{"accountId":"42","npCommunicationId":"NPWR99999_00","trophyId":`+id+`}`); c != 200 {
			t.Fatalf("unlock: %d", c)
		}
	}
	if c := unlock(t, srv, "k", `{"accountId":"42","npCommunicationId":"NPWR99999_00","trophyId":77}`); c != 404 {
		t.Fatalf("unknown trophy: %d", c)
	}
	_, m = get(t, srv, tok, "/api/trophy/v1/users/me/trophyTitles")
	ts := m["trophyTitles"].([]any)
	if len(ts) != 1 {
		t.Fatalf("%v", m)
	}
	tt := ts[0].(map[string]any)
	if tt["progress"].(float64) != 66 || tt["hasTrophyGroups"] != true || tt["earnedTrophies"].(map[string]any)["gold"].(float64) != 1 {
		t.Fatalf("%v", tt)
	}
	_, m = get(t, srv, tok, "/api/trophy/v1/users/42/trophySummary")
	if m["accountId"] != "42" || m["trophyLevel"].(float64) != 1 || m["progress"].(float64) != 52 {
		t.Fatalf("summary %v", m)
	}
	_, m = get(t, srv, tok, "/api/trophy/v1/npCommunicationIds/NPWR99999_00/trophyGroups")
	if len(m["trophyGroups"].([]any)) != 2 {
		t.Fatalf("%v", m)
	}
	_, m = get(t, srv, tok, "/api/trophy/v1/users/me/npCommunicationIds/NPWR99999_00/trophyGroups/all/trophies?limit=2")
	if m["totalItemCount"].(float64) != 3 || m["nextOffset"].(float64) != 2 || len(m["trophies"].([]any)) != 2 {
		t.Fatalf("%v", m)
	}
	if m["trophies"].([]any)[1].(map[string]any)["earned"] != true {
		t.Fatalf("%v", m)
	}
	_, m = get(t, srv, tok, "/api/trophy/v1/users/me/npCommunicationIds/NPWR99999_00/trophyGroups")
	if m["progress"].(float64) != 66 {
		t.Fatalf("%v", m)
	}
	if c, _ := get(t, srv, tok, "/api/trophy/v1/npCommunicationIds/NOPE/trophyGroups"); c != 404 {
		t.Fatal(c)
	}
	if c, _ := get(t, srv, tok, "/api/trophy/v1/something/else"); c != 404 {
		t.Fatal(c)
	}
}
