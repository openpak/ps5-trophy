package store

import (
	"testing"
	"time"

	"openpak/ps5-trophy/internal/fakeachievements"
)

func testSet() Set {
	return Set{NpCommunicationID: "NPWR55555_00", Groups: []Group{{ID: "default", Trophies: []Trophy{
		{ID: 0, Type: Platinum}, {ID: 1, Type: Bronze}}}, {ID: "001", Trophies: []Trophy{{ID: 2, Type: Gold}}}}}
}

// instance is one "process": a fresh store (sets reloaded from disk) over the same core.
func instance(t *testing.T, fake *fakeachievements.Fake) *Achievements {
	t.Helper()
	a, conn, err := DialAchievements("passthrough:///bufnet", fake.Key, nil, fake.Serve(t)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := a.AddSet(testSet()); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestEarnedTrophiesLiveInTheCoreAndSurviveRestart(t *testing.T) {
	fake := fakeachievements.New("0123456789abcdef0123456789abcdef")
	a := instance(t, fake)
	first := time.Unix(1_800_000_000, 0)
	if err := a.Unlock("acct-1", "NPWR55555_00", 1, first); err != nil {
		t.Fatal(err)
	}
	if err := a.Unlock("acct-1", "NPWR55555_00", 2, first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// A repeat does not move the first earn time.
	if err := a.Unlock("acct-1", "NPWR55555_00", 1, first.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if a.Unlock("acct-1", "NPWR00000_99", 1, first) != ErrNoSet || a.Unlock("acct-1", "NPWR55555_00", 9, first) != ErrNoTrophy {
		t.Fatal("unknown set or trophy accepted")
	}

	b := instance(t, fake) // restart: nothing but the core remembers
	e := b.Earnings("acct-1", "NPWR55555_00")
	if len(e) != 2 || !e[1].At.Equal(first) || e[1].Progress != 100 || !e[2].At.Equal(first.Add(time.Hour)) {
		t.Fatalf("%+v", e)
	}
	if p := b.Played("acct-1"); len(p) != 1 || p[0] != "NPWR55555_00" {
		t.Fatal(p)
	}
	if len(b.Earnings("acct-2", "NPWR55555_00")) != 0 || len(b.Played("acct-2")) != 0 {
		t.Fatal("another account sees trophies")
	}
	// Memory holds no earnings in core mode.
	if len(b.Memory.earn) != 0 {
		t.Fatal("earnings kept in process memory")
	}
}

func TestCoreDownIsAWriteError(t *testing.T) {
	fake := fakeachievements.New("0123456789abcdef0123456789abcdef")
	a := instance(t, fake)
	fake.Down = true
	if err := a.Unlock("acct-1", "NPWR55555_00", 1, time.Now()); err == nil {
		t.Fatal("unlock reported success while the core was down")
	}
	if len(a.Earnings("acct-1", "NPWR55555_00")) != 0 {
		t.Fatal("read invented data")
	}
}
