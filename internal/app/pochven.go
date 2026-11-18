package app

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// pochvenLayout defines the Pochven highway topology. The three home systems
// (Kino, Niarja, Archee) sit at the vertices of the large outer triangle;
// the remaining systems trace the three sub-triangles and connecting lines.
var pochvenLayout = []struct {
	name   string
	x, y   float64
	home   bool
	kspace bool // true = k-space entry point, not a Pochven system
}{
	{"Kino", 0, -680, true, false},
	{"Nalvula", 80, -560, false, false},
	{"Otela", -80, -560, false, false},
	{"Konola", 160, -440, false, false},
	{"Krirald", 240, -320, false, false},
	{"Otanuomi", 320, -200, false, false},
	{"Urhinichi", 400, -80, false, false},
	{"Nani", 480, 40, false, false},
	{"Skarkon", 560, 160, false, false},
	{"Raravoss", 640, 280, false, false},
	{"Niarja", 720, 400, true, false},
	{"Harva", 560, 400, false, false},
	{"Tunudan", 400, 400, false, false},
	{"Kuharah", 240, 400, false, false},
	{"Ahtila", 80, 400, false, false},
	{"Senda", -80, 400, false, false},
	{"Wirashoda", -240, 400, false, false},
	{"Ala", -400, 400, false, false},
	{"Vale", -560, 400, false, false},
	{"Archee", -720, 400, true, false},
	{"Angymonne", -640, 280, false, false},
	{"Ignebaener", -160, -440, false, false},
	{"Komo", -240, -320, false, false},
	{"Sakenta", -320, -200, false, false},
	{"Arvasaras", -400, -80, false, false},
	{"Kaunokka", -480, 40, false, false},
	{"Ichoriya", -560, 160, false, false},
}

// pochvenEdges lists every conduit connection by system name.
var pochvenEdges = [][2]string{
	// Top triangle: Kino–Nalvula–Otela
	{"Kino", "Nalvula"}, {"Kino", "Otela"}, {"Nalvula", "Otela"},
	// Right Perun leg: Nalvula → Otanuomi
	{"Nalvula", "Konola"}, {"Konola", "Krirald"}, {"Krirald", "Otanuomi"},
	// Border Perun → Svarog
	{"Otanuomi", "Urhinichi"},
	// Right Svarog leg: Urhinichi → Raravoss
	{"Urhinichi", "Nani"}, {"Nani", "Skarkon"}, {"Skarkon", "Raravoss"},
	// Bottom-right triangle: Raravoss–Niarja–Harva
	{"Raravoss", "Niarja"}, {"Raravoss", "Harva"}, {"Niarja", "Harva"},
	// Bottom Svarag leg: Harva → Ahtila
	{"Harva", "Tunudan"}, {"Tunudan", "Kuharah"}, {"Kuharah", "Ahtila"},
	// Border Svarog → Veles
	{"Ahtila", "Senda"},
	// Bottom Veles leg: Senda → Vale
	{"Senda", "Wirashoda"}, {"Wirashoda", "Ala"}, {"Ala", "Vale"},
	// Bottom-left triangle: Vale–Archee–Angymonne
	{"Vale", "Archee"}, {"Vale", "Angymonne"}, {"Archee", "Angymonne"},
	// Left Veles leg: Angymonne → Arvasaras
	{"Angymonne", "Ichoriya"}, {"Ichoriya", "Kaunokka"}, {"Kaunokka", "Arvasaras"},
	// Border Veles → Perun
	{"Arvasaras", "Sakenta"},
	// Left Perun leg: Sakenta → Otela
	{"Sakenta", "Komo"}, {"Komo", "Ignebaener"}, {"Ignebaener", "Otela"},
}

// SeedPochven places all Pochven highway systems at their correct triangle
// positions and seeds every conduit connection. Guarded by the
// "pochven_topology_seeded" flag so it runs once (or on first upgrade from
// the old constellation-grid layout).
func (a *App) SeedPochven() {
	ctx := context.Background()

	if v, _ := a.store.GetSetting("pochven_topology_seeded"); v == "1" {
		return
	}

	log.Println("SeedPochven: seeding Pochven topology…")

	names := make([]string, len(pochvenLayout))
	for i, n := range pochvenLayout {
		names[i] = n.name
	}

	idMap, err := a.esi.SearchSystems(ctx, names)
	if err != nil {
		log.Println("SeedPochven: name lookup:", err)
		return
	}

	var homeIDs, pochvenIDs []string

	for _, node := range pochvenLayout {
		id, ok := idMap[node.name]
		if !ok {
			log.Printf("SeedPochven: %q not found in ESI — skipping", node.name)
			continue
		}

		info, err := a.esi.GetSystemInfo(ctx, id)
		if err != nil {
			log.Printf("SeedPochven: system info %d (%s): %v", id, node.name, err)
			continue
		}

		s := dbSystemFromInfo(info)
		s.X = node.x
		s.Y = node.y
		if err := a.store.SeedSystem(s); err != nil {
			log.Printf("SeedPochven: seed %s: %v", node.name, err)
			continue
		}

		if node.home {
			homeIDs = append(homeIDs, fmt.Sprintf("%d", id))
		}
		if !node.kspace {
			pochvenIDs = append(pochvenIDs, fmt.Sprintf("%d", id))
		}
	}

	// Seed conduit connections (stable, no expiry).
	for _, edge := range pochvenEdges {
		idA, okA := idMap[edge[0]]
		idB, okB := idMap[edge[1]]
		if !okA || !okB {
			continue
		}
		if err := a.store.UpsertConnection(idA, idB); err != nil {
			log.Printf("SeedPochven: connection %s–%s: %v", edge[0], edge[1], err)
		}
	}

	_ = a.store.SetSetting("home_system_ids", strings.Join(homeIDs, ","))
	_ = a.store.SetSetting("pochven_system_ids", strings.Join(pochvenIDs, ","))
	_ = a.store.SetSetting("pochven_topology_seeded", "1")

	log.Printf("SeedPochven: done — %d home systems, %d Pochven systems", len(homeIDs), len(pochvenIDs))
	runtime.EventsEmit(a.ctx, "map:update")
}
