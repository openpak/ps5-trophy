// Package store holds trophy-set definitions and per-account earnings behind
// the Store interface. Memory is the implementation; sets load from JSON files.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Trophy types as PSN spells them.
const (
	Bronze   = "bronze"
	Silver   = "silver"
	Gold     = "gold"
	Platinum = "platinum"
)

// Trophy is one trophy definition.
type Trophy struct {
	ID                  int    `json:"trophyId"`
	Hidden              bool   `json:"trophyHidden"`
	Type                string `json:"trophyType"`
	Name                string `json:"trophyName"`
	Detail              string `json:"trophyDetail"`
	IconURL             string `json:"trophyIconUrl"`
	SmallIconURL        string `json:"trophySmallIconUrl,omitempty"`
	Rare                int    `json:"trophyRare"`
	EarnedRate          string `json:"trophyEarnedRate"`
	ProgressTargetValue string `json:"trophyProgressTargetValue,omitempty"`
	RewardName          string `json:"trophyRewardName,omitempty"`
	RewardImageURL      string `json:"trophyRewardImageUrl,omitempty"`
}

// Group is a trophy group ("default" holds the base game).
type Group struct {
	ID       string   `json:"trophyGroupId"`
	Name     string   `json:"trophyGroupName"`
	Detail   string   `json:"trophyGroupDetail"`
	IconURL  string   `json:"trophyGroupIconUrl"`
	Trophies []Trophy `json:"trophies"`
}

// Set is the definition of one title's trophy set (one file).
type Set struct {
	NpCommunicationID string  `json:"npCommunicationId"`
	NpServiceName     string  `json:"npServiceName"` // trophy2 for PS5
	Version           string  `json:"trophySetVersion"`
	Name              string  `json:"trophyTitleName"`
	Detail            string  `json:"trophyTitleDetail"`
	IconURL           string  `json:"trophyTitleIconUrl"`
	Platform          string  `json:"trophyTitlePlatform"`
	Groups            []Group `json:"groups"`
}

// Earned is one earned trophy of one account.
type Earned struct {
	At       time.Time
	Progress int
}

// Errors.
var (
	ErrNoSet    = errors.New("no such trophy set")
	ErrNoTrophy = errors.New("no such trophy")
)

// Store is what the API needs.
type Store interface {
	AddSet(Set) error
	Set(npCommID string) (Set, bool)
	Sets() []Set // sorted by npCommunicationId
	Unlock(account, npCommID string, trophyID int, at time.Time) error
	// Earnings returns the account's earned trophies of one set, by trophy id.
	Earnings(account, npCommID string) map[int]Earned
	// Played returns the ids of the sets the account has earned something in.
	Played(account string) []string
}

// Validate checks a set and fills defaults.
func (s *Set) Validate() error {
	if s.NpCommunicationID == "" {
		return errors.New("npCommunicationId missing")
	}
	if s.NpServiceName == "" {
		s.NpServiceName = "trophy2"
	}
	if s.Platform == "" {
		s.Platform = "PS5"
	}
	if s.Version == "" {
		s.Version = "1.00"
	}
	if len(s.Groups) == 0 {
		return errors.New("no groups")
	}
	seenG := map[string]bool{}
	seenT := map[int]bool{}
	for gi := range s.Groups {
		g := &s.Groups[gi]
		if g.ID == "" || g.ID == "all" || seenG[g.ID] {
			return fmt.Errorf("bad or duplicate group id %q", g.ID)
		}
		seenG[g.ID] = true
		for ti := range g.Trophies {
			t := &g.Trophies[ti]
			switch t.Type {
			case Bronze, Silver, Gold, Platinum:
			default:
				return fmt.Errorf("trophy %d: bad type %q", t.ID, t.Type)
			}
			if seenT[t.ID] || t.ID < 0 {
				return fmt.Errorf("bad or duplicate trophy id %d", t.ID)
			}
			seenT[t.ID] = true
			if t.EarnedRate == "" {
				t.EarnedRate = "0.0"
			}
		}
	}
	return nil
}

// LoadDir adds every *.json file in dir to st.
func LoadDir(st interface{ AddSet(Set) error }, dir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return 0, err
	}
	sort.Strings(files)
	for _, f := range files {
		if err := LoadFile(st, f); err != nil {
			return 0, err
		}
	}
	return len(files), nil
}

// LoadFile adds the set in one JSON file to st.
func LoadFile(st interface{ AddSet(Set) error }, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var s Set
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := st.AddSet(s); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Memory is the in-memory Store.
type Memory struct {
	mu   sync.RWMutex
	sets map[string]Set
	earn map[string]map[string]map[int]Earned // account -> set -> trophy
}

// NewMemory returns an empty Memory store.
func NewMemory() *Memory {
	return &Memory{sets: map[string]Set{}, earn: map[string]map[string]map[int]Earned{}}
}

func (m *Memory) AddSet(s Set) error {
	if err := s.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sets[s.NpCommunicationID] = s
	return nil
}

func (m *Memory) Set(id string) (Set, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sets[id]
	return s, ok
}

func (m *Memory) Sets() []Set {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Set, 0, len(m.sets))
	for _, s := range m.sets {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NpCommunicationID < out[j].NpCommunicationID })
	return out
}

func (m *Memory) Unlock(account, id string, trophyID int, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sets[id]
	if !ok {
		return ErrNoSet
	}
	found := false
	for _, g := range s.Groups {
		for _, t := range g.Trophies {
			if t.ID == trophyID {
				found = true
			}
		}
	}
	if !found {
		return ErrNoTrophy
	}
	if m.earn[account] == nil {
		m.earn[account] = map[string]map[int]Earned{}
	}
	if m.earn[account][id] == nil {
		m.earn[account][id] = map[int]Earned{}
	}
	if _, dup := m.earn[account][id][trophyID]; !dup { // first earn time wins
		m.earn[account][id][trophyID] = Earned{At: at.UTC(), Progress: 100}
	}
	return nil
}

func (m *Memory) Earnings(account, id string) map[int]Earned {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[int]Earned{}
	for k, v := range m.earn[account][id] {
		out[k] = v
	}
	return out
}

func (m *Memory) Played(account string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for id, e := range m.earn[account] {
		if len(e) > 0 {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
