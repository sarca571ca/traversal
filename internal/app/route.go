package app

import (
	"context"
	"fmt"
	"math"
)

type RouteJump struct {
	SystemID   int32  `json:"systemId"`
	SystemName string `json:"systemName"`
	JumpType   string `json:"jumpType"` // "gate" or "wormhole"
}

type RouteResult struct {
	Jumps      []RouteJump `json:"jumps"`
	TotalJumps int         `json:"totalJumps"`
}

func (a *App) PlanRoute(origin, destination, flag string, useWormholes bool) (RouteResult, error) {
	ctx := context.Background()

	originID, err := a.esi.SearchSystem(ctx, origin)
	if err != nil {
		return RouteResult{}, fmt.Errorf("origin: %w", err)
	}
	destID, err := a.esi.SearchSystem(ctx, destination)
	if err != nil {
		return RouteResult{}, fmt.Errorf("destination: %w", err)
	}

	if !useWormholes {
		return a.gateRoute(ctx, originID, destID, flag)
	}

	conns, err := a.store.GetAllConnections()
	if err != nil || len(conns) == 0 {
		return a.gateRoute(ctx, originID, destID, flag)
	}

	// Build bidirectional WH adjacency graph from tracked connections.
	whAdj := make(map[int32][]int32)
	for _, c := range conns {
		whAdj[c.SystemA] = append(whAdj[c.SystemA], c.SystemB)
		whAdj[c.SystemB] = append(whAdj[c.SystemB], c.SystemA)
	}

	// K-space nodes in the graph are potential entry/exit points.
	// J-space system IDs fall in the range 31000000–31999999.
	kspaceInGraph := make(map[int32]bool)
	for id := range whAdj {
		if id < 31000000 || id >= 32000000 {
			kspaceInGraph[id] = true
		}
	}
	if len(kspaceInGraph) == 0 {
		return a.gateRoute(ctx, originID, destID, flag)
	}

	// WH set for jump-type annotation.
	whSet := make(map[[2]int32]bool)
	for _, c := range conns {
		whSet[[2]int32{c.SystemA, c.SystemB}] = true
		whSet[[2]int32{c.SystemB, c.SystemA}] = true
	}

	// For every k-space entry node, BFS to all reachable k-space exits.
	// Keep the stitch with the fewest total jumps.
	var bestFull []int32
	bestTotal := math.MaxInt32

	for entryID := range kspaceInGraph {
		leg1, err := a.gateLeg(ctx, originID, entryID, flag)
		if err != nil {
			continue
		}

		for exitID, whPath := range whBFS(whAdj, entryID, kspaceInGraph) {
			leg2, err := a.gateLeg(ctx, exitID, destID, flag)
			if err != nil {
				continue
			}

			total := (len(leg1) - 1) + (len(whPath) - 1) + (len(leg2) - 1)
			if total < bestTotal {
				bestTotal = total
				full := make([]int32, 0, len(leg1)+len(whPath)+len(leg2))
				full = append(full, leg1...)
				full = append(full, whPath[1:]...)  // skip duplicate entry junction
				full = append(full, leg2[1:]...)    // skip duplicate exit junction
				bestFull = full
			}
		}
	}

	if bestFull == nil {
		// No usable WH shortcut found — fall back to pure gate route.
		return a.gateRoute(ctx, originID, destID, flag)
	}

	return a.buildResult(ctx, bestFull, whSet)
}

// gateRoute asks ESI for a pure gate route and builds a RouteResult.
func (a *App) gateRoute(ctx context.Context, originID, destID int32, flag string) (RouteResult, error) {
	route, err := a.esi.GetRoute(ctx, originID, destID, nil, flag)
	if err != nil {
		return RouteResult{}, err
	}
	return a.buildResult(ctx, route, nil)
}

// gateLeg returns the ESI gate route between two systems, or a single-element
// slice when they are the same system (no travel needed).
func (a *App) gateLeg(ctx context.Context, from, to int32, flag string) ([]int32, error) {
	if from == to {
		return []int32{from}, nil
	}
	return a.esi.GetRoute(ctx, from, to, nil, flag)
}

// buildResult resolves system names and annotates each hop as gate or wormhole.
func (a *App) buildResult(ctx context.Context, route []int32, whSet map[[2]int32]bool) (RouteResult, error) {
	nameMap := make(map[int32]string)
	var toResolve []int32
	for _, id := range route {
		if sys, _ := a.store.GetSystem(id); sys != nil {
			nameMap[id] = sys.Name
		} else {
			toResolve = append(toResolve, id)
		}
	}
	if len(toResolve) > 0 {
		if resolved, err := a.esi.ResolveNames(ctx, toResolve); err == nil {
			for id, name := range resolved {
				nameMap[id] = name
			}
		}
	}

	jumps := make([]RouteJump, len(route))
	for i, id := range route {
		jt := "gate"
		if i > 0 && whSet != nil && whSet[[2]int32{route[i-1], id}] {
			jt = "wormhole"
		}
		jumps[i] = RouteJump{
			SystemID:   id,
			SystemName: nameMap[id],
			JumpType:   jt,
		}
	}

	return RouteResult{
		Jumps:      jumps,
		TotalJumps: len(route) - 1,
	}, nil
}

// whBFS finds all k-space systems reachable from start through the WH graph.
// Returns a map of exitID → full path (start inclusive, exit inclusive).
func whBFS(adj map[int32][]int32, start int32, kspaceNodes map[int32]bool) map[int32][]int32 {
	type node struct {
		id   int32
		path []int32
	}

	result := make(map[int32][]int32)
	visited := map[int32]bool{start: true}
	queue := []node{{id: start, path: []int32{start}}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, nb := range adj[cur.id] {
			if visited[nb] {
				continue
			}
			visited[nb] = true
			path := make([]int32, len(cur.path)+1)
			copy(path, cur.path)
			path[len(cur.path)] = nb
			queue = append(queue, node{id: nb, path: path})

			if kspaceNodes[nb] {
				result[nb] = path
			}
		}
	}
	return result
}
