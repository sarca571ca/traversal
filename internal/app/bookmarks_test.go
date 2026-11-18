package app

import (
	"testing"
	"time"
)

const samplePaste = ` P-TYY-Krirald    Coordinate    No gate-to-gate route    Anher    Barvigrard    Metropolis    2026.11.18 00:52    2026.11.19 00:52    Rajisa Solette
 P2P-ORE-Krirald	Coordinate	12	Ala	Krai Veles	Pochven	2026.11.18 00:50	2026.11.19 00:50	Rajisa Solette
 W-CDJ-[Sleepers]    Coordinate    0    Krirald    Krai Perun    Pochven    2026.11.17 23:38    2026.11.18 23:38    Rajisa Solette
 N-TLG-[Drones]    Coordinate    0    Krirald    Krai Perun    Pochven    2026.11.17 23:35    2026.11.18 03:35    Rajisa Solette
 Safe spot    Coordinate    0    Krirald    Krai Perun    Pochven    2026.11.17 23:35    2026.11.18 03:35    Rajisa Solette
`

func TestParseBookmarks(t *testing.T) {
	bms, skipped := ParseBookmarks(samplePaste)
	if len(bms) != 4 || len(skipped) != 1 {
		t.Fatalf("got %d bookmarks, %d skipped: %v", len(bms), len(skipped), skipped)
	}
	b := bms[0]
	if b.Prefix != "P" || b.SigID != "TYY" || b.Dest != "Krirald" || b.System != "Anher" || b.DestUnknown {
		t.Errorf("bad first bookmark: %+v", b)
	}
	if !b.Expires.Equal(time.Date(2026, 11, 19, 0, 52, 0, 0, time.UTC)) {
		t.Errorf("bad expiry: %v", b.Expires)
	}
	if bms[1].System != "Ala" || bms[1].SigID != "ORE" {
		t.Errorf("tab-separated row parsed wrong: %+v", bms[1])
	}
	if !bms[3].DestUnknown || bms[3].Dest != "Drones" || bms[3].System != "Krirald" {
		t.Errorf("bad unknown-dest bookmark: %+v", bms[3])
	}
}

func TestMergeSig(t *testing.T) {
	cases := [][3]string{
		{"", "ABC", "ABC"},
		{"ABC-123", "ABC", "ABC-123"},
		{"XYZ", "ABC", "ABC"},
		{"ABC", "", "ABC"},
	}
	for _, c := range cases {
		if got := mergeSig(c[0], c[1]); got != c[2] {
			t.Errorf("mergeSig(%q,%q)=%q want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestSpaceClass(t *testing.T) {
	pochven := map[int32]bool{30000001: true}
	cases := []struct {
		id   int32
		sec  float32
		want string
	}{
		{31000005, -1, spaceJ},
		{30000001, -1, spacePochven},
		{30000002, 0.46, spaceHigh}, // displays as 0.5
		{30000002, 0.44, spaceLow},
		{30000002, 0.05, spaceLow},  // displays as 0.1
		{30000002, 0.04, spaceNull}, // displays as 0.0
		{30000002, -0.3, spaceNull},
	}
	for _, c := range cases {
		if got := spaceClass(c.id, c.sec, pochven); got != c.want {
			t.Errorf("spaceClass(%d, %v) = %s, want %s", c.id, c.sec, got, c.want)
		}
	}
}

func TestParseStubNotes(t *testing.T) {
	cases := [][3]string{
		{"→ J-space · Sleepers (bm expires 11-18 23:38)", "J-space", "Sleepers"},
		{"→ Nullsec · Drones", "Nullsec", "Drones"},
		{"→ Sleepers (bm expires 11-18 23:38)", "Sleepers", ""},
	}
	for _, c := range cases {
		d, n := parseStubNotes(c[0])
		if d != c[1] || n != c[2] {
			t.Errorf("parseStubNotes(%q) = %q,%q want %q,%q", c[0], d, n, c[1], c[2])
		}
	}
}

func TestPPPrefix(t *testing.T) {
	bms, _ := ParseBookmarks(" PP-ABC-Krirald\tCoordinate\t0\tAla\tKrai Veles\tPochven\t2026.11.18 00:50\t2026.11.19 00:50\tX")
	if len(bms) != 1 || bms[0].Prefix != "PP" {
		t.Fatalf("got %+v", bms)
	}
	if bmPrefixes["PP"] != bmPrefixes["P2P"] {
		t.Error("PP should mean the same as P2P")
	}
}
