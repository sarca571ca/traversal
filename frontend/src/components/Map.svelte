<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import {
    GetMapData, UpdateSystemPosition, GetViewport, SaveViewport, AddManualConnection,
    GetHomeSystemIDs, GetPochvenSystemIDs, ClearMap
  } from '../../wailsjs/go/app/App'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import type { app } from '../../wailsjs/go/models'
  import WHPanel from './WHPanel.svelte'
  import SigPanel from './SigPanel.svelte'

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

  function connColor(mass: string, urgent: boolean): string {
    if (urgent)              return '#ff7043'
    if (mass === 'critical') return '#cc4444'
    if (mass === 'half')     return '#ddaa22'
    return '#c0cce0'
  }

  type Line = {
    id: number, ax: number, ay: number, bx: number, by: number,
    color: string, eol: boolean, urgent: boolean, label: string
  }

  $: lines = (() => {
    const posMap = new Map(systems.map(s => [s.id, s]))
    const now = Math.floor(Date.now() / 1000)
    return connections.reduce<Line[]>((acc, conn) => {
      const a = posMap.get(conn.systemA)
      const b = posMap.get(conn.systemB)
      if (a && b) {
        const urgent = conn.expiresAt > 0 && conn.expiresAt > now && (conn.expiresAt - now) < 4 * 3600
        const types = [conn.whTypeA, conn.whTypeB].filter(Boolean)
        const label = types.length === 2 ? types.join('/') : types[0] ?? ''
        acc.push({
          id: conn.id,
          ax: a.x, ay: a.y,
          bx: b.x, by: b.y,
          color: connColor(conn.massStatus, urgent),
          eol: conn.eol,
          urgent,
          label,
        })
      }
      return acc
    }, [])
  })()

  $: routeSystemIDs = new Set(routeJumps?.map(j => j.systemId) ?? [])

  type RouteEdge = { a: { x: number, y: number }, b: { x: number, y: number } }
  $: routeEdges = (() => {
    if (!routeJumps || routeJumps.length < 2) return [] as RouteEdge[]
    const posMap = new Map(systems.map(s => [s.id, s]))
    const edges: RouteEdge[] = []
    for (let i = 0; i < routeJumps.length - 1; i++) {
      const a = posMap.get(routeJumps[i].systemId)
      const b = posMap.get(routeJumps[i + 1].systemId)
      if (a && b) edges.push({ a, b })
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
        <line
          x1={line.ax} y1={line.ay} x2={line.bx} y2={line.by}
          stroke={isSelConn ? '#e8c840' : line.color}
          stroke-width={isSelConn ? 3 / scale : 2 / scale}
          stroke-dasharray={line.eol ? `${6/scale},${4/scale}` : 'none'}
          opacity={isSelConn ? 1 : 0.7}
          class:conn-urgent={line.urgent && !isSelConn}
        />
        {#if line.label}
          <text
            x={(line.ax + line.bx) / 2}
            y={(line.ay + line.by) / 2 - 7 / scale}
            text-anchor="middle"
            font-size={10 / scale}
            fill={isSelConn ? '#e8c840' : line.color}
            opacity="0.9"
          >{line.label}</text>
        {/if}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <line
          x1={line.ax} y1={line.ay} x2={line.bx} y2={line.by}
          stroke="transparent"
          stroke-width={16 / scale}
          style:cursor="pointer"
          on:click={(e) => onConnClick(e, line.id)}
        />
      {/each}

      <!-- Route overlay -->
      {#each routeEdges as edge}
        <line
          x1={edge.a.x} y1={edge.a.y} x2={edge.b.x} y2={edge.b.y}
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
