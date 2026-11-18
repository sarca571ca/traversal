package app

import (
	"context"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"traversal/internal/db"
)

// Bookmark is one row of an in-game People & Places bookmark copy.
//
// Columns (tab separated): Label, Type, Jumps, System, Constellation, Region,
// Created, Expiry, Creator. Labels follow the "<PREFIX>-<SIG>-<DEST>" naming
// convention, e.g. "P2P-ORE-Krirald" = sig ORE in this system leads to
// Krirald. A bracketed destination ("W-CDJ-[Sleepers]") means the far side
// system is not known, so only a signature is recorded.
type Bookmark struct {
	Label       string
	Prefix      string
	SigID       string
	Dest        string // destination system name, or the bracket note when DestUnknown
	DestUnknown bool
	System      string // system the bookmark sits in
	Created     time.Time
	Expires     time.Time
}

// Space classes a bookmark prefix can refer to.
const (
	spaceHigh    = "Highsec"
	spaceLow     = "Lowsec"
	spaceNull    = "Nullsec"
	spacePochven = "Pochven"
	spaceJ       = "J-space"
)

// bmPrefixes maps label prefixes to the space class of the destination and,
// where the prefix implies it, of the bookmark's own system ("" = any).
//
//	P2P/PP  Pochven → Pochven     P  known space → Pochven
//	H    → highsec   L → lowsec   N → nullsec   W → J-space
var bmPrefixes = map[string]struct{ src, dst string }{
	"P2P": {spacePochven, spacePochven},
	"PP":  {spacePochven, spacePochven},
	"P":   {"kspace", spacePochven},
	"H":   {"", spaceHigh},
	"L":   {"", spaceLow},
	"N":   {"", spaceNull},
	"W":   {"", spaceJ},
}

var (
	bmFieldSep = regexp.MustCompile(`\t+| {2,}`)
	bmLabel    = regexp.MustCompile(`^([A-Za-z0-9]+)-([A-Za-z]{3}(?:-\d{3})?)-(.+)$`)
	bmDate     = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2} \d{2}:\d{2}$`)
)

const bmDateLayout = "2006.01.02 15:04" // EVE time == UTC

// ParseBookmarks parses pasted bookmark rows. Lines that don't look like
// wormhole bookmarks are returned in skipped with a reason.
func ParseBookmarks(text string) (bms []Bookmark, skipped []string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := bmFieldSep.Split(line, -1)
		if len(f) < 4 {
			skipped = append(skipped, fmt.Sprintf("%q: not a bookmark row", line))
			continue
		}
		label := strings.TrimSpace(f[0])
		m := bmLabel.FindStringSubmatch(label)
		if m == nil {
			skipped = append(skipped, fmt.Sprintf("%s: label isn't PREFIX-SIG-DEST", label))
			continue
		}
		bm := Bookmark{
			Label:  label,
			Prefix: strings.ToUpper(m[1]),
			SigID:  strings.ToUpper(m[2]),
			Dest:   strings.TrimSpace(m[3]),
			System: strings.TrimSpace(f[3]),
		}
		if strings.HasPrefix(bm.Dest, "[") && strings.HasSuffix(bm.Dest, "]") {
			bm.DestUnknown = true
			bm.Dest = strings.TrimSpace(bm.Dest[1 : len(bm.Dest)-1])
		}
		var dates []time.Time
		for _, v := range f[4:] {
			v = strings.TrimSpace(v)
			if bmDate.MatchString(v) {
				if t, err := time.ParseInLocation(bmDateLayout, v, time.UTC); err == nil {
					dates = append(dates, t)
				}
			}
		}
		if len(dates) > 0 {
			bm.Created = dates[0]
		}
		if len(dates) > 1 {
			bm.Expires = dates[1]
		}
		bms = append(bms, bm)
	}
	return bms, skipped
}

// BookmarkImportResult summarises an import for the UI.
type BookmarkImportResult struct {
	Added      int      `json:"added"`      // new connections
	Updated    int      `json:"updated"`    // existing connections updated
	Systems    int      `json:"systems"`    // systems added to the map
	Signatures int      `json:"signatures"` // unknown-destination sigs recorded
	Skipped    []string `json:"skipped"`
	Warnings   []string `json:"warnings"` // imported, but the prefix disagrees with the systems
}

// ImportBookmarks parses an in-game bookmark paste and fills the map from it.
// Existing connections get their sig IDs and expiry updated; mass, ship size,
// WH type and EOL that were set by hand are kept.
func (a *App) ImportBookmarks(text string) (BookmarkImportResult, error) {
	ctx := context.Background()
	res := BookmarkImportResult{Skipped: []string{}, Warnings: []string{}}

	bms, skipped := ParseBookmarks(text)
	res.Skipped = append(res.Skipped, skipped...)
	if len(bms) == 0 {
		return res, fmt.Errorf("no wormhole bookmarks found in paste")
	}

	now := time.Now()
	live := bms[:0]
	for _, bm := range bms {
		if !bm.Expires.IsZero() && bm.Expires.Before(now) {
			res.Skipped = append(res.Skipped, bm.Label+": bookmark already expired")
			continue
		}
		live = append(live, bm)
	}
	bms = live

	ids, err := a.resolveSystemNames(ctx, bms)
	if err != nil {
		return res, err
	}

	// Group the bookmarks by the wormhole they describe. Both sides of the
	// same hole (e.g. P-YKJ-Urhinichi in Anttiri + H-ADY-Anttiri in
	// Urhinichi) collapse into one connection.
	type side struct {
		sig     string
		created time.Time
	}
	type hole struct {
		a, b    int32 // a < b
		sigA    side
		sigB    side
		expires time.Time
		latest  time.Time
		labels  []string
	}
	holes := map[[2]int32]*hole{}
	var order [][2]int32
	type resolved struct {
		bm       Bookmark
		src, dst int32
	}
	var toCheck []resolved

	for _, bm := range bms {
		src, ok := ids[strings.ToLower(bm.System)]
		if !ok {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: unknown system %q", bm.Label, bm.System))
			continue
		}
		if _, known := bmPrefixes[bm.Prefix]; !known {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: unknown prefix %q", bm.Label, bm.Prefix))
		}
		if bm.DestUnknown {
			toCheck = append(toCheck, resolved{bm, src, 0})
			if err := a.recordUnknownSig(src, bm); err != nil {
				return res, err
			}
			res.Signatures++
			continue
		}
		dst, ok := ids[strings.ToLower(bm.Dest)]
		if !ok {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: unknown system %q", bm.Label, bm.Dest))
			continue
		}
		if src == dst {
			res.Skipped = append(res.Skipped, bm.Label+": leads to its own system")
			continue
		}
		a.clearResolvedStub(src, bm.SigID)
		lo, hi := src, dst
		if lo > hi {
			lo, hi = hi, lo
		}
		key := [2]int32{lo, hi}
		h := holes[key]
		if h == nil {
			h = &hole{a: lo, b: hi}
			holes[key] = h
			order = append(order, key)
		}
		s := side{sig: bm.SigID, created: bm.Created}
		if src == lo {
			if h.sigA.sig == "" || !s.created.Before(h.sigA.created) {
				h.sigA = s
			}
		} else {
			if h.sigB.sig == "" || !s.created.Before(h.sigB.created) {
				h.sigB = s
			}
		}
		// The most recently created bookmark has the freshest expiry estimate.
		if !bm.Expires.IsZero() && (h.expires.IsZero() || !bm.Created.Before(h.latest)) {
			h.expires = bm.Expires
			h.latest = bm.Created
		}
		h.labels = append(h.labels, bm.Label)
		toCheck = append(toCheck, resolved{bm, src, dst})
	}

	// Place systems that aren't on the map yet next to whatever they connect
	// to. Repeat until nothing changes so chains of new systems fan out.
	onMap := func(id int32) bool { s, _ := a.store.GetSystem(id); return s != nil }
	for progress := true; progress; {
		progress = false
		for _, key := range order {
			ina, inb := onMap(key[0]), onMap(key[1])
			switch {
			case ina && !inb:
				if a.addSystemNear(ctx, key[1], key[0]) == nil {
					res.Systems++
					progress = true
				}
			case inb && !ina:
				if a.addSystemNear(ctx, key[0], key[1]) == nil {
					res.Systems++
					progress = true
				}
			}
		}
	}
	// Anything left is an island disconnected from the existing map.
	for _, key := range order {
		for i, id := range key {
			if !onMap(id) {
				if err := a.addSystemNear(ctx, id, key[1-i]); err != nil {
					log.Printf("ImportBookmarks: add system %d: %v", id, err)
					continue
				}
				res.Systems++
			}
		}
	}

	// Cross-check prefixes against the systems now that they're all on the map.
	pochven := map[int32]bool{}
	for _, id := range a.GetPochvenSystemIDs() {
		pochven[id] = true
	}
	for _, r := range toCheck {
		if w := a.checkPrefix(r.bm, r.src, r.dst, pochven); w != "" {
			res.Warnings = append(res.Warnings, w)
		}
	}

	conduits := a.conduitPairs()

	for _, key := range order {
		h := holes[key]
		if !onMap(h.a) || !onMap(h.b) {
			res.Skipped = append(res.Skipped, strings.Join(h.labels, ", ")+": could not add system to map")
			continue
		}
		existing, err := a.store.GetConnectionBetween(h.a, h.b)
		if err != nil {
			return res, err
		}
		if existing == nil {
			if err := a.store.UpsertConnection(h.a, h.b); err != nil {
				return res, err
			}
			if existing, err = a.store.GetConnectionBetween(h.a, h.b); err != nil || existing == nil {
				return res, fmt.Errorf("create connection: %v", err)
			}
			res.Added++
		} else {
			res.Updated++
		}

		sigA := mergeSig(existing.SigIDA, h.sigA.sig)
		sigB := mergeSig(existing.SigIDB, h.sigB.sig)
		var exp int64
		// Never put a timer on a Pochven conduit, or the expiry sweep
		// would delete a permanent highway edge.
		if !h.expires.IsZero() && !conduits[key] {
			exp = h.expires.Unix()
		}
		if err := a.store.SetConnectionSigs(existing.ID, sigA, sigB, exp); err != nil {
			return res, err
		}
	}

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "map:update")
	}
	return res, nil
}

// spaceClass classifies a system the way bookmark prefixes do.
func spaceClass(id int32, security float32, pochven map[int32]bool) string {
	switch {
	case id >= 31000000 && id < 32000000:
		return spaceJ
	case pochven[id]:
		return spacePochven
	}
	sec := math.Round(float64(security)*10) / 10 // as shown in game
	switch {
	case sec >= 0.5:
		return spaceHigh
	case sec > 0:
		return spaceLow
	default:
		return spaceNull
	}
}

// checkPrefix returns a warning when a bookmark's prefix contradicts the
// systems it was found in / leads to (dst 0 = unknown destination).
func (a *App) checkPrefix(bm Bookmark, src, dst int32, pochven map[int32]bool) string {
	want, ok := bmPrefixes[bm.Prefix]
	if !ok {
		return ""
	}
	if want.src != "" {
		if s, _ := a.store.GetSystem(src); s != nil {
			got := spaceClass(s.ID, s.Security, pochven)
			if want.src == "kspace" && (got == spacePochven || got == spaceJ) {
				return fmt.Sprintf("%s: %s prefix expects known space, but %s is %s", bm.Label, bm.Prefix, s.Name, got)
			}
			if want.src != "kspace" && got != want.src {
				return fmt.Sprintf("%s: %s prefix expects %s, but %s is %s", bm.Label, bm.Prefix, want.src, s.Name, got)
			}
		}
	}
	if dst != 0 {
		if s, _ := a.store.GetSystem(dst); s != nil {
			if got := spaceClass(s.ID, s.Security, pochven); got != want.dst {
				return fmt.Sprintf("%s: %s prefix means → %s, but %s is %s", bm.Label, bm.Prefix, want.dst, s.Name, got)
			}
		}
	}
	return ""
}

// mergeSig keeps a more specific existing sig ID ("ABC-123") over the
// bookmark's 3-letter form ("ABC"), otherwise takes the bookmark's.
func mergeSig(existing, incoming string) string {
	if incoming == "" {
		return existing
	}
	if strings.HasPrefix(strings.ToUpper(existing), incoming) {
		return existing
	}
	return incoming
}

// resolveSystemNames maps lowercased system names in the bookmarks to IDs,
// using the local DB first and a single ESI call for the rest.
func (a *App) resolveSystemNames(ctx context.Context, bms []Bookmark) (map[string]int32, error) {
	ids := map[string]int32{}
	var missing []string
	seen := map[string]bool{}
	want := func(name string) {
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			return
		}
		seen[key] = true
		if s, _ := a.store.GetSystemByName(name); s != nil {
			ids[key] = s.ID
			return
		}
		missing = append(missing, name)
	}
	for _, bm := range bms {
		want(bm.System)
		if !bm.DestUnknown {
			want(bm.Dest)
		}
	}
	if len(missing) == 0 {
		return ids, nil
	}
	found, err := a.esi.SearchSystems(ctx, missing)
	if err != nil {
		return nil, fmt.Errorf("look up systems: %w", err)
	}
	for name, id := range found {
		ids[strings.ToLower(name)] = id
	}
	return ids, nil
}

// addSystemNear fetches a system from ESI and places it beside anchorID.
func (a *App) addSystemNear(ctx context.Context, id, anchorID int32) error {
	info, err := a.esi.GetSystemInfo(ctx, id)
	if err != nil {
		return err
	}
	s := dbSystemFromInfo(info)
	s.X, s.Y = a.placeNode(anchorID)
	return a.store.UpsertSystem(s)
}

// conduitPairs returns the seeded Pochven conduit edges keyed by (lo, hi) ID.
func (a *App) conduitPairs() map[[2]int32]bool {
	out := map[[2]int32]bool{}
	for _, e := range pochvenEdges {
		sa, _ := a.store.GetSystemByName(e[0])
		sb, _ := a.store.GetSystemByName(e[1])
		if sa == nil || sb == nil {
			continue
		}
		lo, hi := sa.ID, sb.ID
		if lo > hi {
			lo, hi = hi, lo
		}
		out[[2]int32{lo, hi}] = true
	}
	return out
}

// recordUnknownSig stores a bookmark whose far side isn't known as a
// wormhole signature in its system, e.g. "→ Drones".
func (a *App) recordUnknownSig(systemID int32, bm Bookmark) error {
	sigID := bm.SigID
	if sigs, err := a.store.ListSignaturesBySystem(systemID); err == nil {
		for _, s := range sigs {
			if strings.HasPrefix(strings.ToUpper(s.SigID), sigID) {
				sigID = s.SigID // keep the full "ABC-123" if already scanned
				break
			}
		}
	}
	notes := "→ " + bm.Dest
	if p, ok := bmPrefixes[bm.Prefix]; ok {
		notes = "→ " + p.dst
		if bm.Dest != "" {
			notes += " · " + bm.Dest
		}
	}
	if !bm.Expires.IsZero() {
		notes += " (bm expires " + bm.Expires.Format("01-02 15:04") + ")"
	}
	_, err := a.store.UpsertSignature(db.Signature{
		SolarSystemID: systemID,
		SigID:         sigID,
		SigType:       "Wormhole", // matches the casing SigPanel uses
		Notes:         notes,
		ExpiresAt:     bm.Expires,
	})
	return err
}

var stubExpirySuffix = regexp.MustCompile(`\s*\(bm expires [^)]*\)\s*$`)

// parseStubNotes splits "→ J-space · Sleepers (bm expires …)" into
// ("J-space", "Sleepers").
func parseStubNotes(notes string) (dest, note string) {
	s := strings.TrimSpace(strings.TrimPrefix(notes, "→"))
	s = stubExpirySuffix.ReplaceAllString(s, "")
	if i := strings.Index(s, "·"); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len("·"):])
	}
	return s, ""
}

func (a *App) whStubs() []WHStub {
	sigs, err := a.store.ListUnresolvedWormholes()
	if err != nil {
		log.Println("whStubs:", err)
		return []WHStub{}
	}
	offsets, err := a.store.StubOffsets()
	if err != nil {
		log.Println("whStubs offsets:", err)
	}
	out := make([]WHStub, 0, len(sigs))
	for _, s := range sigs {
		dest, note := parseStubNotes(s.Notes)
		st := WHStub{ID: s.ID, SystemID: s.SolarSystemID, SigID: s.SigID, Dest: dest, Note: note}
		if !s.ExpiresAt.IsZero() {
			st.ExpiresAt = s.ExpiresAt.Unix()
		}
		if o, ok := offsets[s.ID]; ok {
			st.Placed, st.Dx, st.Dy = true, o[0], o[1]
		}
		out = append(out, st)
	}
	return out
}

// clearResolvedStub removes an unknown-destination stub for sigID in a system
// once a bookmark names where that sig actually goes.
func (a *App) clearResolvedStub(systemID int32, sigID string) {
	sigs, err := a.store.ListSignaturesBySystem(systemID)
	if err != nil {
		return
	}
	for _, s := range sigs {
		if strings.HasPrefix(strings.ToUpper(s.SigID), sigID) &&
			strings.EqualFold(string(s.SigType), "wormhole") && strings.HasPrefix(s.Notes, "→") {
			_ = a.store.DeleteSignature(s.ID)
		}
	}
}
