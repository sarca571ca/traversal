<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import {
    GetMapData, UpdateSystemPosition, GetViewport, SaveViewport, AddManualConnection,
    GetHomeSystemIDs, GetPochvenSystemIDs, ClearMap, SetStubPosition } from '../../wailsjs/go/app/App'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import type { app } from '../../wailsjs/go/models'
  import WHPanel from './WHPanel.svelte'
  import SigPanel from './SigPanel.svelte'
  import BookmarkImport from './BookmarkImport.svelte'

  let showImport = false

  export let routeJumps: app.RouteJump[] | null = null

  type SystemNode = app.SystemNode
  type MapConnection = app.MapConnection
  type RouteJump = app.RouteJump

  const NODE_R     = 26
  const GRID       = 40
  const MIN_SCALE  = 0.15
  const MAX_SCALE  = 4
  // Equilateral triangle inscribed in circle of radius NODE_R, pointing up.
  const TRI_HALF   = (NODE_R * Math.sqrt(3) / 2).toFixed(2)
  const TRI_LOW    = (NODE_R * 0.5).toFixed(2)
  const TRI_POINTS = `0,${-NODE_R} ${TRI_HALF},${TRI_LOW} -${TRI_HALF},${TRI_LOW}`

  let svgEl: SVGSVGElement
  let tx = 0, ty = 0, scale = 1
  let systems: SystemNode[]    = []
  let connections: MapConnection[] = []
  let stubs: app.WHStub[] = []
  let selectedIDs    = new Set<number>()
  let selectedConnID: number | null = null
  let sigSystemID:    number | null = null
  let centred          = false
  let viewportRestored = false
  let snapEnabled      = false

  // Home/Pochven system ID sets (loaded once from Go settings)
  let homeIDs    = new Set<number>()
  let pochvenIDs = new Set<number>()

  // Pan state
  let panning   = false
  let panOrigin = { x: 0, y: 0 }

  // Drag state
  let draggingID:     number | null = null
  let dragOrigin      = { x: 0, y: 0 }
  let dragNodeStart   = { x: 0, y: 0 }
  let dragGroupStart  = new Map<number, { x: number; y: number }>()
  let didDrag         = false
  let preMousedownIDs = new Set<number>() // selection snapshot taken before mousedown mutates it

  // Rubber-band multi-select state
  let rubberBanding     = false
  let justRubberBanded  = false
  let rbStart   = { x: 0, y: 0 }
  let rbCurrent = { x: 0, y: 0 }

  // Connect mode
  let connectMode = false
  let connectSrc: number | null = null

  // Viewport save debounce
  let saveTimer: ReturnType<typeof setTimeout> | null = null
  function scheduleSaveViewport() {
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(() => SaveViewport(tx, ty, scale), 400)
  }

  function snap(v: number): number {
    return snapEnabled ? Math.round(v / GRID) * GRID : v
  }

  async function load() {
    const data = await GetMapData()
    const newSystems: SystemNode[] = data.systems ?? []
    connections = data.connections ?? []
    stubs = data.stubs ?? []

    systems = newSystems.map(s => {
      const existing = systems.find(e => e.id === s.id)
      return existing ? { ...s, x: existing.x, y: existing.y } : s
    })

    if (!centred && systems.length > 0) {
      if (!viewportRestored) centerView()
      centred = true
    }
  }

  function centerView() {
    if (!svgEl || systems.length === 0) return
    const rect = svgEl.getBoundingClientRect()
    const xs = systems.map(s => s.x), ys = systems.map(s => s.y)
    const cx = (Math.min(...xs) + Math.max(...xs)) / 2
    const cy = (Math.min(...ys) + Math.max(...ys)) / 2
    tx = rect.width / 2 - cx * scale
    ty = rect.height / 2 - cy * scale
    scheduleSaveViewport()
  }

  function onWheel(e: WheelEvent) {
    e.preventDefault()
    const rect = svgEl.getBoundingClientRect()
    const mx = e.clientX - rect.left
    const my = e.clientY - rect.top
    const factor = e.deltaY > 0 ? 0.9 : 1.1
    const newScale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, scale * factor))
    tx = mx + (tx - mx) * (newScale / scale)
    ty = my + (ty - my) * (newScale / scale)
    scale = newScale
    scheduleSaveViewport()
  }

  function onBgMouseDown(e: MouseEvent) {
    if (e.button !== 0 || draggingID !== null) return
    if (e.shiftKey) {
      // Shift+drag on background starts rubber-band multi-select
      const r = svgEl.getBoundingClientRect()
      rubberBanding = true
      rbStart   = { x: e.clientX - r.left, y: e.clientY - r.top }
      rbCurrent = { ...rbStart }
    } else {
      panning   = true
      panOrigin = { x: e.clientX - tx, y: e.clientY - ty }
    }
  }

  function onNodeMouseDown(e: MouseEvent, id: number) {
    e.stopPropagation()
    if (e.button !== 0) return
    if (connectMode) return

    // Snapshot selection before we mutate it — onNodeClick uses this to decide toggle vs open.
    preMousedownIDs = new Set(selectedIDs)

    // Shift+mousedown: add to selection so drag includes this node.
    // No-shift outside selection: exclusive-select for clean single-drag.
    if (e.shiftKey) {
      if (!selectedIDs.has(id)) selectedIDs = new Set([...selectedIDs, id])
    } else if (!selectedIDs.has(id)) {
      selectedIDs = new Set([id])
    }

    draggingID = id
    didDrag    = false
    dragOrigin = { x: e.clientX, y: e.clientY }

    // Capture start positions for every selected node (group drag).
    dragGroupStart = new Map(
      systems.filter(s => selectedIDs.has(s.id)).map(s => [s.id, { x: s.x, y: s.y }])
    )
    // Safety fallback if dragged node somehow isn't in the group map.
    if (!dragGroupStart.has(id)) {
      const node = systems.find(s => s.id === id)!
      dragNodeStart = { x: node.x, y: node.y }
    }
  }

  function onMouseMove(e: MouseEvent) {
    if (draggingStub) {
      const ddx = (e.clientX - stubDragOrigin.x) / scale
      const ddy = (e.clientY - stubDragOrigin.y) / scale
      if (Math.hypot(ddx, ddy) > 3 / scale) stubDidDrag = true
      const off = { dx: draggingStub.start.dx + ddx, dy: draggingStub.start.dy + ddy }
      // Don't let the stub end sit inside its own system node.
      const len = Math.hypot(off.dx, off.dy)
      const min = NODE_R + STUB_R + 4
      if (len < min) {
        const k = min / (len || 1)
        off.dx = len ? off.dx * k : 0
        off.dy = len ? off.dy * k : -min
      }
      stubOverride = new Map(stubOverride).set(draggingStub.id, off)
      return
    }
    if (panning) {
      tx = e.clientX - panOrigin.x
      ty = e.clientY - panOrigin.y
    }
    if (rubberBanding) {
      const r = svgEl.getBoundingClientRect()
      rbCurrent = { x: e.clientX - r.left, y: e.clientY - r.top }
    }
    if (draggingID !== null) {
      const dx = (e.clientX - dragOrigin.x) / scale
      const dy = (e.clientY - dragOrigin.y) / scale
      if (Math.hypot(dx, dy) > 3 / scale) didDrag = true

      if (dragGroupStart.size > 1) {
        // Multi-node drag: apply uniform delta to every node in the group.
        systems = systems.map(s => {
          const orig = dragGroupStart.get(s.id)
          return orig ? { ...s, x: snap(orig.x + dx), y: snap(orig.y + dy) } : s
        })
      } else {
        const orig = dragGroupStart.get(draggingID) ?? dragNodeStart
        systems = systems.map(s =>
          s.id === draggingID
            ? { ...s, x: snap(orig.x + dx), y: snap(orig.y + dy) }
            : s
        )
      }
    }
  }

  async function onMouseUp() {
    if (draggingStub) {
      const id  = draggingStub.id
      const off = stubOverride.get(id)
      draggingStub = null
      if (stubDidDrag && off) {
        await SetStubPosition(id, off.dx, off.dy)
        await load()
      }
      stubOverride = new Map([...stubOverride].filter(([k]) => k !== id))
      return
    }
    if (rubberBanding) {
      const minX = Math.min(rbStart.x, rbCurrent.x)
      const maxX = Math.max(rbStart.x, rbCurrent.x)
      const minY = Math.min(rbStart.y, rbCurrent.y)
      const maxY = Math.max(rbStart.y, rbCurrent.y)
      // Only commit selection if the rect has real size (avoid stray clicks).
      if (maxX - minX > 4 || maxY - minY > 4) {
        justRubberBanded = true  // suppress the bg-click that fires right after mouseup
        const hits = new Set<number>()
        for (const s of systems) {
          // Convert world coords → SVG viewport coords for hit test.
          const sx = s.x * scale + tx
          const sy = s.y * scale + ty
          if (sx >= minX && sx <= maxX && sy >= minY && sy <= maxY) hits.add(s.id)
        }
        if (hits.size > 0) { selectedIDs = hits; sigSystemID = null }
      }
      rubberBanding = false
    }

    if (draggingID !== null && didDrag) {
      // Persist new positions for all nodes that moved.
      const movedIDs = dragGroupStart.size > 1 ? [...dragGroupStart.keys()] : [draggingID]
      const moved    = systems.filter(s => movedIDs.includes(s.id))
      await Promise.all(moved.map(s => UpdateSystemPosition(s.id, s.x, s.y)))
    }

    if (panning) scheduleSaveViewport()
    panning    = false
    draggingID = null
  }

  async function onNodeClick(e: MouseEvent, id: number) {
    e.stopPropagation()
    // If a drag just finished, don't treat the mouseup-click as a selection toggle.
    if (didDrag) { didDrag = false; return }

    if (connectMode) {
      if (connectSrc === null) {
        connectSrc = id
      } else if (connectSrc !== id) {
        await AddManualConnection(connectSrc, id)
        connectMode = false
        connectSrc  = null
      } else {
        connectSrc = null
      }
      return
    }

    if (e.shiftKey) {
      // Shift+click: toggle this node in the selection without changing sig panel.
      const next = new Set(selectedIDs)
      if (next.has(id)) { next.delete(id) } else { next.add(id) }
      selectedIDs = next
      return
    }

    // Plain click: open sig panel, or deselect if this node was the sole selection before mousedown.
    if (preMousedownIDs.size === 1 && preMousedownIDs.has(id)) {
      selectedIDs = new Set()
      sigSystemID = null
    } else {
      selectedIDs    = new Set([id])
      sigSystemID    = id
      selectedConnID = null
    }
  }

  function onStubClick(e: MouseEvent, systemId: number) {
    e.stopPropagation()
    if (stubDidDrag) { stubDidDrag = false; return }
    selectedIDs    = new Set([systemId])
    sigSystemID    = systemId
    selectedConnID = null
  }

  function onConnClick(e: MouseEvent, id: number) {
    e.stopPropagation()
    selectedConnID = selectedConnID === id ? null : id
    selectedIDs    = new Set()
    sigSystemID    = null
  }

  function onBgClick() {
    if (justRubberBanded) { justRubberBanded = false; return }
    selectedIDs    = new Set()
    selectedConnID = null
    sigSystemID    = null
    if (connectMode) { connectMode = false; connectSrc = null }
  }

  function secColor(sec: number, id: number): string {
    if (id >= 31000000 && id < 32000000) return '#40c880'  // J-space
    if (pochvenIDs.has(id))              return '#c46bff'  // Pochven
    if (sec >= 0.5) return '#4488dd'                        // High-sec
    if (sec > 0.0)  return '#ddaa22'                        // Low-sec
    return '#cc4444'                                        // Null-sec
  }

  // Wormholes with a timer are coloured by time left, matching the
  // 24h / 4h / 1h timer buckets: green >4h, yellow 1–4h, red ≤1h.
  function whTimeColor(secsLeft: number): string {
    if (secsLeft > 4 * 3600) return '#40c880'
    if (secsLeft > 3600)     return '#ddaa22'
    return '#cc4444'
  }

  // Re-evaluate timer colours every 30s even when nothing else changes.
  let nowTick = Date.now()
  const nowTimer = setInterval(() => nowTick = Date.now(), 30_000)
  onDestroy(() => clearInterval(nowTimer))

  function connColor(mass: string, urgent: boolean): string {
    if (urgent)              return '#ff7043'
    if (mass === 'critical') return '#cc4444'
    if (mass === 'half')     return '#ddaa22'
    return '#c0cce0'
  }

  const STUB_LEN = 78
  const STUB_SPREAD = Math.PI / 7
  const STUB_R = NODE_R / 2  // half a system node

  // Stub drag state. stubOverride holds the live offset while dragging so
  // the stub moves without waiting for a map reload.
  let draggingStub: { id: number, start: { dx: number, dy: number } } | null = null
  let stubDragOrigin = { x: 0, y: 0 }
  let stubDidDrag = false
  let stubOverride = new Map<number, { dx: number, dy: number }>()

  function onStubMouseDown(e: MouseEvent, st: StubLine) {
    e.stopPropagation()
    if (e.button !== 0 || connectMode) return
    draggingStub   = { id: st.id, start: { dx: st.bx - st.nx, dy: st.by - st.ny } }
    stubDragOrigin = { x: e.clientX, y: e.clientY }
    stubDidDrag    = false
  }

  function stubColor(dest: string): string {
    switch (dest) {
      case 'J-space': return '#40c880'
      case 'Pochven': return '#c46bff'
      case 'Highsec': return '#4488dd'
      case 'Lowsec':  return '#ddaa22'
      case 'Nullsec': return '#cc4444'
      default:        return '#8899aa'
    }
  }

  type StubLine = {
    id: number, systemId: number, nx: number, ny: number,
    ax: number, ay: number, bx: number, by: number, ux: number, uy: number,
    color: string, sigId: string, label: string, urgent: boolean,
  }

  // Unknown-destination wormholes fan out from their system on the side
  // facing away from its existing connections.
  $: stubLines = (() => {
    const posMap = new Map(systems.map(s => [s.id, s]))
    const now = Math.floor(Date.now() / 1000)
    const bySystem = new Map<number, app.WHStub[]>()
    for (const st of stubs) {
      if (!posMap.has(st.systemId)) continue
      bySystem.set(st.systemId, [...(bySystem.get(st.systemId) ?? []), st])
    }
    const out: StubLine[] = []
    for (const [sysId, list] of bySystem) {
      const n = posMap.get(sysId)!
      let vx = 0, vy = 0
      for (const c of connections) {
        const other = c.systemA === sysId ? c.systemB : c.systemB === sysId ? c.systemA : null
        const o = other !== null ? posMap.get(other) : undefined
        if (!o) continue
        const d = Math.hypot(o.x - n.x, o.y - n.y) || 1
        vx += (o.x - n.x) / d
        vy += (o.y - n.y) / d
      }
      const base = (vx === 0 && vy === 0) ? -Math.PI / 2 : Math.atan2(-vy, -vx)
      list.forEach((st, i) => {
        let dx: number, dy: number
        const ov = stubOverride.get(st.id)
        if (ov) {
          ({ dx, dy } = ov)
        } else if (st.placed) {
          dx = st.dx; dy = st.dy
        } else {
          const ang = base + (i - (list.length - 1) / 2) * STUB_SPREAD
          dx = Math.cos(ang) * (NODE_R + STUB_LEN)
          dy = Math.sin(ang) * (NODE_R + STUB_LEN)
        }
        const len = Math.hypot(dx, dy) || 1
        const ux = dx / len, uy = dy / len
        out.push({
          id: st.id, systemId: sysId, nx: n.x, ny: n.y,
          ax: n.x + ux * NODE_R, ay: n.y + uy * NODE_R,
          bx: n.x + dx, by: n.y + dy,
          ux, uy,
          color: stubColor(st.dest),
          sigId: st.sigId,
          label: [st.dest, st.note].filter(Boolean).join(' · '),
          urgent: st.expiresAt > 0 && st.expiresAt - now < 4 * 3600,
        })
      })
    }
    return out
  })()

  type Line = {
    id: number, systemA: number, systemB: number,
    d: string, lx: number, ly: number,
    color: string, eol: boolean, urgent: boolean, label: string
  }

  // Wormholes between two Pochven systems are drawn as arcs so they don't
  // sit on top of the straight conduit lines. The arc bows in towards the
  // centre of the Pochven triangle, so a hole along a leg curves inside it.
  function connGeometry(
    a: { x: number, y: number }, b: { x: number, y: number }, arc: boolean,
    centre: { x: number, y: number } | null,
  ): { d: string, lx: number, ly: number } {
    const mx = (a.x + b.x) / 2, my = (a.y + b.y) / 2
    if (!arc) return { d: `M ${a.x} ${a.y} L ${b.x} ${b.y}`, lx: mx, ly: my }
    const len = Math.hypot(b.x - a.x, b.y - a.y) || 1
    const nx = -(b.y - a.y) / len, ny = (b.x - a.x) / len
    let k = Math.min(160, Math.max(40, len * 0.22))
    let sign = 1
    if (centre) {
      // Signed distance from the chord's midpoint to the centre, along the normal.
      const toCentre = (centre.x - mx) * nx + (centre.y - my) * ny
      sign = toCentre < 0 ? -1 : 1
      // Don't bow past the centre on chords that already cut through the middle.
      k = Math.min(k, Math.max(40, Math.abs(toCentre) * 0.8))
    }
    const cx = mx + nx * k * sign, cy = my + ny * k * sign
    return {
      d: `M ${a.x} ${a.y} Q ${cx} ${cy} ${b.x} ${b.y}`,
      // Point on the curve at t = 0.5.
      lx: 0.25 * a.x + 0.5 * cx + 0.25 * b.x,
      ly: 0.25 * a.y + 0.5 * cy + 0.25 * b.y,
    }
  }

  $: lines = (() => {
    const posMap = new Map(systems.map(s => [s.id, s]))
    const now = Math.floor(nowTick / 1000)
    // Triangle centre = middle of the three home corners; fall back to all Pochven.
    const homes = systems.filter(s => homeIDs.has(s.id))
    const pSys = homes.length === 3 ? homes : systems.filter(s => pochvenIDs.has(s.id))
    const centre = pSys.length
      ? { x: pSys.reduce((t, s) => t + s.x, 0) / pSys.length, y: pSys.reduce((t, s) => t + s.y, 0) / pSys.length }
      : null
    return connections.reduce<Line[]>((acc, conn) => {
      const a = posMap.get(conn.systemA)
      const b = posMap.get(conn.systemB)
      if (a && b) {
        // Gate/conduit connections and untimed ones keep the old colouring.
        const timed  = !conn.conduit && conn.expiresAt > 0
        const urgent = timed && conn.expiresAt > now && (conn.expiresAt - now) <= 3600
        const types = [conn.sigIdA, conn.sigIdB].filter(Boolean)
        const label = types.length === 2 ? types.join('/') : types[0] ?? ''
        const arc = !conn.conduit && pochvenIDs.has(conn.systemA) && pochvenIDs.has(conn.systemB)
        acc.push({
          id: conn.id,
          systemA: conn.systemA, systemB: conn.systemB,
          ...connGeometry(a, b, arc, centre),
          color: timed ? whTimeColor(conn.expiresAt - now) : connColor(conn.massStatus, urgent),
          eol: conn.eol,
          urgent,
          label,
        })
      }
      return acc
    }, [])
  })()

  $: routeSystemIDs = new Set(routeJumps?.map(j => j.systemId) ?? [])

  // Route overlay follows the drawn connection (arc or straight) when there is one.
  $: routeEdges = (() => {
    if (!routeJumps || routeJumps.length < 2) return [] as string[]
    const posMap = new Map(systems.map(s => [s.id, s]))
    const byPair = new Map(lines.map(l => [`${Math.min(l.systemA, l.systemB)}-${Math.max(l.systemA, l.systemB)}`, l.d]))
    const edges: string[] = []
    for (let i = 0; i < routeJumps.length - 1; i++) {
      const ia = routeJumps[i].systemId, ib = routeJumps[i + 1].systemId
      const a = posMap.get(ia), b = posMap.get(ib)
      if (!a || !b) continue
      edges.push(byPair.get(`${Math.min(ia, ib)}-${Math.max(ia, ib)}`) ?? `M ${a.x} ${a.y} L ${b.x} ${b.y}`)
    }
    return edges
  })()

  async function clearMap() {
    if (!confirm('Clear the map? This will remove all systems and connections except the Pochven highway layout.')) return
    try {
      await ClearMap()
      selectedIDs    = new Set()
      selectedConnID = null
      sigSystemID    = null
    } catch (e: any) {
      alert('Clear failed: ' + (e?.toString() ?? 'unknown error'))
    }
  }

  $: sigSystemNode = sigSystemID !== null ? systems.find(s => s.id === sigSystemID) : null

  let unsubscribe: (() => void) | null = null

  onMount(async () => {
    const vp = await GetViewport()
    if (vp && (vp.tx !== 0 || vp.ty !== 0 || vp.scale !== 1)) {
      tx = vp.tx
      ty = vp.ty
      scale = vp.scale
      viewportRestored = true
    }
    const [hids, pids] = await Promise.all([GetHomeSystemIDs(), GetPochvenSystemIDs()])
    homeIDs    = new Set(hids ?? [])
    pochvenIDs = new Set(pids ?? [])
    await load()
    unsubscribe = EventsOn('map:update', load)
  })

  onDestroy(() => {
    if (unsubscribe) unsubscribe()
    if (saveTimer) clearTimeout(saveTimer)
  })
</script>

<div
  class="map-wrap"
  on:mousemove={onMouseMove}
  on:mouseup={onMouseUp}
  on:mouseleave={onMouseUp}
>
  {#if systems.length === 0}
    <div class="empty-state">
      <div class="empty-icon">◎</div>
      <p class="empty-title">No systems mapped</p>
      <p class="empty-sub">Add a character — your current location will appear automatically.</p>
    </div>
  {/if}

  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <svg
    bind:this={svgEl}
    class="map-svg"
    on:wheel={onWheel}
    on:mousedown={onBgMouseDown}
    on:click={onBgClick}
    style:cursor={connectMode ? (connectSrc ? 'crosshair' : 'cell') : rubberBanding ? 'crosshair' : panning ? 'grabbing' : 'grab'}
  >
    <defs>
      <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse"
        patternTransform="translate({tx % 40},{ty % 40})">
        <path d="M 40 0 L 0 0 0 40" fill="none" stroke="var(--border)" stroke-width="0.5" opacity="0.5"/>
      </pattern>
      <filter id="glow" x="-50%" y="-50%" width="200%" height="200%">
        <feGaussianBlur stdDeviation="3" result="blur"/>
        <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
      </filter>
    </defs>

    <rect width="100%" height="100%" fill="url(#grid)"/>

    <g transform="translate({tx},{ty}) scale({scale})">

      <!-- Edges -->
      {#each lines as line (line.id)}
        {@const isSelConn = selectedConnID === line.id}
        <path
          d={line.d}
          fill="none"
          stroke={isSelConn ? '#e8c840' : line.color}
          stroke-width={isSelConn ? 3 / scale : 2 / scale}
          stroke-dasharray={line.eol ? `${6/scale},${4/scale}` : 'none'}
          opacity={isSelConn ? 1 : 0.7}
          class:conn-urgent={line.urgent && !isSelConn}
        />
        {#if line.label}
          <text
            x={line.lx}
            y={line.ly - 7 / scale}
            text-anchor="middle"
            font-size={10 / scale}
            fill={isSelConn ? '#e8c840' : line.color}
            opacity="0.9"
          >{line.label}</text>
        {/if}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <path
          d={line.d}
          fill="none"
          stroke="transparent"
          stroke-width={16 / scale}
          style:cursor="pointer"
          on:click={(e) => onConnClick(e, line.id)}
        />
      {/each}

      <!-- Unknown-destination wormhole stubs -->
      {#each stubLines as st (st.id)}
        <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
        <g class="stub"
          style:cursor={draggingStub?.id === st.id ? 'grabbing' : 'pointer'}
          on:click={(e) => onStubClick(e, st.systemId)}
          on:mousedown={(e) => onStubMouseDown(e, st)}
        >
          <title>{st.sigId} → {st.label || 'unknown'} (click for signatures, drag to move)</title>
          <line
            x1={st.ax} y1={st.ay} x2={st.bx} y2={st.by}
            stroke={st.color}
            stroke-width={1.5 / scale}
            stroke-dasharray="{5/scale},{4/scale}"
            opacity="0.75"
            class:conn-urgent={st.urgent}
          />
          <line x1={st.ax} y1={st.ay} x2={st.bx} y2={st.by} stroke="transparent" stroke-width={14 / scale} />
          <circle cx={st.bx} cy={st.by} r={STUB_R} fill="#0d1824" stroke={st.color}
            stroke-width={1.5 / scale} stroke-dasharray="{4/scale},{3/scale}" />
          <text x={st.bx} y={st.by + 4 / scale} text-anchor="middle" font-size={12 / scale} fill={st.color}>?</text>
          <text
            x={(st.ax + st.bx) / 2 - st.uy * 9}
            y={(st.ay + st.by) / 2 + st.ux * 9 + 3 / scale}
            text-anchor="middle" font-size={10 / scale} fill={st.color} opacity="0.9"
          >{st.sigId}</text>
          <text
            x={st.bx + st.ux * (STUB_R + 6)}
            y={st.by + st.uy * (STUB_R + 6) + 3.5 / scale}
            text-anchor={Math.abs(st.ux) < 0.3 ? 'middle' : st.ux > 0 ? 'start' : 'end'}
            font-size={10 / scale} fill={st.color} opacity="0.85"
          >{st.label}</text>
        </g>
      {/each}

      <!-- Route overlay -->
      {#each routeEdges as d}
        <path
          {d}
          fill="none"
          stroke="#e8c840"
          stroke-width={3 / scale}
          opacity="0.6"
        />
      {/each}

      <!-- Nodes -->
      {#each systems as node (node.id)}
        {@const color = secColor(node.security, node.id)}
        {@const isSelected = selectedIDs.has(node.id)}
        {@const isConnSrc = connectSrc === node.id}
        {@const onRoute = routeSystemIDs.has(node.id)}
        {@const hasChars = node.chars && node.chars.length > 0}

        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <g
          transform="translate({node.x},{node.y})"
          style:cursor={connectMode ? 'pointer' : draggingID === node.id ? 'grabbing' : 'pointer'}
          on:mousedown={(e) => onNodeMouseDown(e, node.id)}
          on:click={(e) => onNodeClick(e, node.id)}
        >
          {#if onRoute}
            <circle r={NODE_R + 8} fill="none"
              stroke="#e8c840" stroke-width={2 / scale} opacity="0.5"
              stroke-dasharray="{4/scale},{3/scale}"/>
          {/if}
          {#if isSelected}
            <circle r={NODE_R + 6} fill="none"
              stroke={color} stroke-width={1.5 / scale} opacity="0.35"/>
          {/if}
          {#if isConnSrc}
            <circle r={NODE_R + 9} fill="none"
              stroke="#e8c840" stroke-width={2 / scale} opacity="0.8"
              stroke-dasharray="{3/scale},{3/scale}"/>
          {/if}

          {#if homeIDs.has(node.id)}
            <polygon
              points={TRI_POINTS}
              fill="#0d1824"
              stroke={color}
              stroke-width={(isSelected || isConnSrc) ? 2.5 / scale : 1.5 / scale}
              filter={hasChars ? 'url(#glow)' : 'none'}
            />
          {:else}
            <circle
              r={NODE_R}
              fill="#0d1824"
              stroke={color}
              stroke-width={(isSelected || isConnSrc) ? 2.5 / scale : 1.5 / scale}
              filter={hasChars ? 'url(#glow)' : 'none'}
            />
          {/if}

          {#if hasChars}
            <circle
              r={8} cx={NODE_R - 3} cy={-(NODE_R - 3)}
              fill="var(--accent)" stroke="#0d1824" stroke-width={1 / scale}
            />
            <text
              x={NODE_R - 3} y={-(NODE_R - 7)}
              text-anchor="middle"
              font-size={9 / scale}
              fill="#0d1824"
              font-weight="bold"
            >{node.chars.length}</text>
          {/if}

          <text
            y={5 / scale}
            text-anchor="middle"
            font-size={10 / scale}
            fill={color}
            opacity="0.85"
          >{node.security >= 0 ? node.security.toFixed(1) : '?'}</text>

          <text
            y={NODE_R + 15 / scale}
            text-anchor="middle"
            font-size={11 / scale}
            fill="var(--text-primary)"
          >{node.name}</text>
        </g>
      {/each}

    </g>

    {#if rubberBanding}
      <rect
        x={Math.min(rbStart.x, rbCurrent.x)}
        y={Math.min(rbStart.y, rbCurrent.y)}
        width={Math.abs(rbCurrent.x - rbStart.x)}
        height={Math.abs(rbCurrent.y - rbStart.y)}
        fill="rgba(100,150,255,0.07)"
        stroke="rgba(100,150,255,0.55)"
        stroke-width="1"
        stroke-dasharray="4,3"
        pointer-events="none"
      />
    {/if}
  </svg>

  <!-- Map toolbar (bottom-left) -->
  <div class="map-toolbar">
    <button
      class="tool-btn"
      on:click|stopPropagation={centerView}
      title="Re-center view"
    >⊹</button>
    <button
      class="tool-btn"
      class:active={connectMode}
      on:click|stopPropagation={() => {
        connectMode = !connectMode
        connectSrc = null
      }}
      title={connectMode ? 'Cancel connect mode' : 'Manually connect two systems'}
    >⊗</button>
    <button
      class="tool-btn"
      class:active={snapEnabled}
      on:click|stopPropagation={() => snapEnabled = !snapEnabled}
      title={snapEnabled ? 'Grid snap on (click to disable)' : 'Grid snap off (click to enable)'}
    >⊞</button>
    <button
      class="tool-btn"
      class:active={showImport}
      on:click|stopPropagation={() => showImport = !showImport}
      title="Import bookmarks from a paste"
    >⎘</button>
    <button
      class="tool-btn tool-btn-danger"
      on:click|stopPropagation={clearMap}
      title="Clear map (keeps Pochven layout)"
    >⌫</button>
  </div>

  {#if connectMode}
    <div class="connect-hint">
      {#if connectSrc === null}
        Click first system…
      {:else}
        {@const srcNode = systems.find(s => s.id === connectSrc)}
        Click second system to connect from {srcNode?.name ?? connectSrc}
      {/if}
    </div>
  {:else if selectedIDs.size > 1}
    <div class="connect-hint">
      {selectedIDs.size} systems selected — drag any to move group · Shift+drag empty space to re-select
    </div>
  {/if}

  {#if selectedConnID !== null}
    {@const selConn = connections.find(c => c.id === selectedConnID)}
    {#if selConn}
      {@const sysA = systems.find(s => s.id === selConn.systemA)}
      {@const sysB = systems.find(s => s.id === selConn.systemB)}
      <WHPanel
        conn={selConn}
        nameA={sysA?.name ?? ''}
        nameB={sysB?.name ?? ''}
        on:close={() => selectedConnID = null}
        on:saved={load}
        on:deleted={() => { selectedConnID = null; load() }}
      />
    {/if}
  {/if}

  {#if showImport}
    <BookmarkImport on:close={() => showImport = false} />
  {/if}

  {#if sigSystemID !== null && sigSystemNode}
    <SigPanel
      systemID={sigSystemID}
      systemName={sigSystemNode.name}
      systemSecurity={sigSystemNode.security}
      on:close={() => { sigSystemID = null; selectedIDs = new Set() }}
    />
  {/if}
</div>

<style>
  .map-wrap {
    position: relative;
    background: var(--bg-base);
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .map-svg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .empty-state {
    position: relative;
    z-index: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    pointer-events: none;
    user-select: none;
  }

  .empty-icon   { font-size: 48px; color: var(--text-dim); line-height: 1; margin-bottom: 4px; }
  .empty-title  { font-size: 13px; font-weight: 600; color: var(--text-secondary); letter-spacing: 0.05em; }
  .empty-sub    { font-size: 11px; color: var(--text-dim); text-align: center; max-width: 260px; line-height: 1.6; }

  .map-toolbar {
    position: absolute;
    bottom: 14px;
    left: 14px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    z-index: 5;
  }

  .tool-btn {
    width: 30px; height: 30px;
    display: flex; align-items: center; justify-content: center;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 5px;
    font-size: 16px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s;
    box-shadow: 0 2px 8px rgba(0,0,0,0.4);
  }
  .tool-btn:hover { color: var(--text-primary); border-color: var(--border-bright); }
  .tool-btn.active {
    background: var(--accent-dim);
    border-color: var(--accent);
    color: var(--accent);
  }
  .tool-btn-danger:hover {
    border-color: rgba(208,64,64,0.6);
    color: var(--danger);
  }

  .connect-hint {
    position: absolute;
    bottom: 54px;
    left: 50%;
    transform: translateX(-50%);
    background: var(--bg-panel);
    border: 1px solid var(--accent-dim);
    border-radius: 4px;
    padding: 5px 12px;
    font-size: 11px;
    color: var(--accent);
    pointer-events: none;
    white-space: nowrap;
    box-shadow: 0 2px 8px rgba(0,0,0,0.4);
  }

  :global(.conn-urgent) {
    animation: connPulse 1.4s ease-in-out infinite;
  }

  @keyframes connPulse {
    0%, 100% { stroke-opacity: 0.8; }
    50%       { stroke-opacity: 0.15; }
  }
</style>
