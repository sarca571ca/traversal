package app

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type SystemNode struct {
	ID       int32   `json:"id"`
	Name     string  `json:"name"`
	Security float64 `json:"security"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Chars    []int64 `json:"chars"`
}

type MapConnection struct {
	ID         int64  `json:"id"`
	SystemA    int32  `json:"systemA"`
	SystemB    int32  `json:"systemB"`
	WHTypeA    string `json:"whTypeA"`
	WHTypeB    string `json:"whTypeB"`
	SigIDA     string `json:"sigIdA"`
	SigIDB     string `json:"sigIdB"`
	ShipSize   string `json:"shipSize"`
	MassStatus string `json:"massStatus"`
	EOL        bool   `json:"eol"`
	ExpiresAt  int64  `json:"expiresAt"`
}

type MapData struct {
	Systems     []SystemNode    `json:"systems"`
	Connections []MapConnection `json:"connections"`
}

func (a *App) GetMapData() MapData {
	systems, err := a.store.GetAllSystems()
	if err != nil {
		log.Println("GetMapData systems:", err)
		return MapData{}
	}

	conns, err := a.store.GetAllConnections()
	if err != nil {
		log.Println("GetMapData connections:", err)
		return MapData{}
	}

	chars, _ := a.store.ListCharacters()
	charsBySystem := make(map[int32][]int64)
	for _, c := range chars {
		if c.CurrentSystemID != 0 {
			charsBySystem[c.CurrentSystemID] = append(charsBySystem[c.CurrentSystemID], c.ID)
		}
	}

	nodes := make([]SystemNode, len(systems))
	for i, s := range systems {
		nodes[i] = SystemNode{
			ID:       s.ID,
			Name:     s.Name,
			Security: float64(s.Security),
			X:        s.X,
			Y:        s.Y,
			Chars:    charsBySystem[s.ID],
		}
		if nodes[i].Chars == nil {
			nodes[i].Chars = []int64{}
		}
	}

	mapConns := make([]MapConnection, len(conns))
	for i, c := range conns {
		mapConns[i] = MapConnection{
			ID:         c.ID,
			SystemA:    c.SystemA,
			SystemB:    c.SystemB,
			WHTypeA:    c.WHTypeA,
			WHTypeB:    c.WHTypeB,
			SigIDA:     c.SigIDA,
			SigIDB:     c.SigIDB,
			ShipSize:   c.ShipSize,
			MassStatus: c.MassStatus,
			EOL:        c.EOL,
			ExpiresAt:  c.ExpiresAt,
		}
	}

	return MapData{Systems: nodes, Connections: mapConns}
}

func (a *App) UpdateSystemPosition(id int32, x, y float64) error {
	return a.store.UpdateSystemPosition(id, x, y)
}

func (a *App) UpdateConnection(id int64, whTypeA, whTypeB, sigIdA, sigIdB, shipSize, massStatus string, eol bool, timerHours int) error {
	return a.store.UpdateConnection(id, whTypeA, whTypeB, sigIdA, sigIdB, shipSize, massStatus, eol, timerHours)
}

func (a *App) DeleteConnection(id int64) error {
	return a.store.DeleteConnection(id)
}

// AddManualConnection creates a connection between two systems without travelling through it.
func (a *App) AddManualConnection(systemA, systemB int32) error {
	if err := a.store.UpsertConnection(systemA, systemB); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "map:update")
	return nil
}

// MapViewport holds the saved pan/zoom state for the map canvas.
type MapViewport struct {
	TX    float64 `json:"tx"`
	TY    float64 `json:"ty"`
	Scale float64 `json:"scale"`
}

func (a *App) GetViewport() MapViewport {
	v := MapViewport{Scale: 1}
	if s, _ := a.store.GetSetting("viewport_tx"); s != "" {
		v.TX, _ = strconv.ParseFloat(s, 64)
	}
	if s, _ := a.store.GetSetting("viewport_ty"); s != "" {
		v.TY, _ = strconv.ParseFloat(s, 64)
	}
	if s, _ := a.store.GetSetting("viewport_scale"); s != "" {
		if f, _ := strconv.ParseFloat(s, 64); f > 0 {
			v.Scale = f
		}
	}
	return v
}

// GetHomeSystemIDs returns IDs of the three Pochven home-corner systems (triangle nodes).
func (a *App) GetHomeSystemIDs() []int32 {
	v, _ := a.store.GetSetting("home_system_ids")
	return parseIDList(v)
}

// GetPochvenSystemIDs returns IDs of all seeded Pochven systems (for purple coloring).
func (a *App) GetPochvenSystemIDs() []int32 {
	v, _ := a.store.GetSetting("pochven_system_ids")
	return parseIDList(v)
}

func parseIDList(s string) []int32 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	ids := make([]int32, 0, len(parts))
	for _, p := range parts {
		if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && v != 0 {
			ids = append(ids, int32(v))
		}
	}
	return ids
}

// GetSystemStatics returns the static wormhole type codes for a J-space system via ESI.
func (a *App) GetSystemStatics(id int32) []string {
	ctx := context.Background()
	statics, err := a.esi.GetSystemStatics(ctx, id)
	if err != nil {
		log.Printf("GetSystemStatics %d: %v", id, err)
		return nil
	}
	if statics == nil {
		return []string{}
	}
	return statics
}

// ClearMap removes all non-Pochven systems, connections, and signatures from the map.
func (a *App) ClearMap() error {
	keepSet := make(map[int32]bool)
	// Primary: IDs stored in settings by SeedPochven.
	for _, id := range a.GetHomeSystemIDs() {
		keepSet[id] = true
	}
	for _, id := range a.GetPochvenSystemIDs() {
		keepSet[id] = true
	}
	// Safety net: look up every Pochven system by name in case settings are incomplete.
	for _, node := range pochvenLayout {
		if sys, _ := a.store.GetSystemByName(node.name); sys != nil {
			keepSet[sys.ID] = true
		}
	}

	if len(keepSet) == 0 {
		return fmt.Errorf("no Pochven systems found — run SeedPochven first")
	}

	keepIDs := make([]int32, 0, len(keepSet))
	for id := range keepSet {
		keepIDs = append(keepIDs, id)
	}

	if err := a.store.DeleteConnectionsExcept(keepIDs); err != nil {
		return err
	}
	if err := a.store.DeleteSignaturesExcept(keepIDs); err != nil {
		return err
	}
	if err := a.store.DeleteSystemsExcept(keepIDs); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "map:update")
	return nil
}

func (a *App) SaveViewport(tx, ty, scale float64) error {
	_ = a.store.SetSetting("viewport_tx", strconv.FormatFloat(tx, 'f', 4, 64))
	_ = a.store.SetSetting("viewport_ty", strconv.FormatFloat(ty, 'f', 4, 64))
	return a.store.SetSetting("viewport_scale", strconv.FormatFloat(scale, 'f', 4, 64))
}

func (a *App) pollLocations() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			changed := a.refreshLocations()

			// Collect expired connections for notification before deleting them.
			expired, err := a.store.GetExpiredConnections()
			if err != nil {
				log.Println("get expired connections:", err)
			}
			if len(expired) > 0 {
				notices := make([]string, 0, len(expired))
				for _, c := range expired {
					nameA := fmt.Sprintf("%d", c.SystemA)
					nameB := fmt.Sprintf("%d", c.SystemB)
					if s, _ := a.store.GetSystem(c.SystemA); s != nil {
						nameA = s.Name
					}
					if s, _ := a.store.GetSystem(c.SystemB); s != nil {
						nameB = s.Name
					}
					notices = append(notices, nameA+" ↔ "+nameB)
				}
				if _, err := a.store.DeleteExpiredConnections(); err != nil {
					log.Println("delete expired connections:", err)
				}
				runtime.EventsEmit(a.ctx, "wh:expired", notices)
				changed = true
			}

			if changed {
				runtime.EventsEmit(a.ctx, "map:update")
			}
		case <-a.ctx.Done():
			return
		}
	}
}

// refreshLocations polls ESI for each character's current system,
// detects transitions, adds new systems, creates connections.
// Returns true if anything changed.
func (a *App) refreshLocations() bool {
	ctx := context.Background()
	chars, err := a.store.ListCharacters()
	if err != nil {
		log.Println("refreshLocations:", err)
		return false
	}

	changed := false
	for _, char := range chars {
		newSystemID, tok, err := a.esi.GetCharacterLocation(ctx, char.ID, char.AccessToken, char.RefreshToken, char.TokenExpiry)
		if err != nil {
			log.Printf("refreshLocations char %d: %v", char.ID, err)
			continue
		}

		// Persist refreshed tokens so the next poll doesn't re-expire.
		if tok.AccessToken != char.AccessToken {
			if err := a.store.UpdateTokens(char.ID, tok.AccessToken, tok.RefreshToken, tok.Expiry); err != nil {
				log.Printf("refreshLocations save tokens char %d: %v", char.ID, err)
			}
		}

		prevSystemID := char.CurrentSystemID

		if newSystemID == prevSystemID {
			continue
		}

		// Ensure the new system exists in our map.
		if sys, _ := a.store.GetSystem(newSystemID); sys == nil {
			info, err := a.esi.GetSystemInfo(ctx, newSystemID)
			if err != nil {
				log.Printf("refreshLocations system info %d: %v", newSystemID, err)
				continue
			}
			s := dbSystemFromInfo(info)
			s.X, s.Y = a.placeNode(prevSystemID)
			if err := a.store.UpsertSystem(s); err != nil {
				log.Printf("refreshLocations upsert system %d: %v", newSystemID, err)
				continue
			}
		}

		// Create a connection between the two systems if we have a previous location.
		if prevSystemID != 0 {
			if err := a.store.UpsertConnection(prevSystemID, newSystemID); err != nil {
				log.Printf("refreshLocations upsert connection: %v", err)
			}
		}

		if err := a.store.SetCharacterSystem(char.ID, newSystemID); err != nil {
			log.Printf("refreshLocations set system: %v", err)
		}

		changed = true
	}
	return changed
}

// placeNode returns (x, y) for a new node connected from sourceID.
// It tries 8 candidate angles at a fixed distance and picks the one
// furthest from all existing nodes.
func (a *App) placeNode(sourceID int32) (float64, float64) {
	const dist = 160.0
	const minSep = 110.0

	source, _ := a.store.GetSystem(sourceID)
	if source == nil {
		return 0, 0
	}

	existing, _ := a.store.GetAllSystems()

	bestX, bestY := source.X+dist, source.Y
	bestClear := -1.0

	for i := 0; i < 8; i++ {
		angle := float64(i) * math.Pi / 4
		cx := source.X + dist*math.Cos(angle)
		cy := source.Y + dist*math.Sin(angle)

		// Find nearest existing node to this candidate.
		nearest := math.MaxFloat64
		for _, s := range existing {
			dx := cx - s.X
			dy := cy - s.Y
			d := math.Sqrt(dx*dx + dy*dy)
			if d < nearest {
				nearest = d
			}
		}

		if nearest >= minSep && nearest > bestClear {
			bestClear = nearest
			bestX, bestY = cx, cy
		}
	}

	return bestX, bestY
}
