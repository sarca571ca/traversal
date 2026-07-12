<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { PlanRoute } from '../../wailsjs/go/app/App'
  import type { app } from '../../wailsjs/go/models'

  type RouteJump = app.RouteJump

  const dispatch = createEventDispatcher<{ route: RouteJump[] | null }>()

  let origin = ''
  let destination = ''
  let flag: 'shortest' | 'secure' | 'insecure' = 'shortest'
  let useWormholes = true
  function setFlag(f: string) { flag = f as typeof flag }

  let loading = false
  let error = ''
  let jumps: RouteJump[] = []
  let totalJumps = 0

  $: gateCount = jumps.slice(1).filter(j => j.jumpType === 'gate').length
  $: whCount = jumps.filter(j => j.jumpType === 'wormhole').length

  async function calculate() {
    if (!origin.trim() || !destination.trim()) return
    loading = true
    error = ''
    jumps = []
    dispatch('route', null)

    try {
      const result = await PlanRoute(origin.trim(), destination.trim(), flag, useWormholes)
      jumps = result.jumps ?? []
      totalJumps = result.totalJumps ?? 0
      dispatch('route', jumps)
    } catch (e: any) {
      error = e?.toString() ?? 'Route calculation failed'
    } finally {
      loading = false
    }
  }

  function clear() {
    jumps = []
    error = ''
    dispatch('route', null)
  }
</script>

<section class="route-section">
  <div class="section-header">
    <span class="section-label">ROUTE PLANNER</span>
    {#if jumps.length > 0}
      <button class="clear-btn" on:click={clear} title="Clear route">×</button>
    {/if}
  </div>

  <div class="inputs">
    <input
      class="system-input"
      placeholder="Origin"
      bind:value={origin}
      on:keydown={(e) => e.key === 'Enter' && calculate()}
    />
    <input
      class="system-input"
      placeholder="Destination"
      bind:value={destination}
      on:keydown={(e) => e.key === 'Enter' && calculate()}
    />
  </div>

  <div class="options">
    <label class="opt-label">
      <input type="checkbox" bind:checked={useWormholes} />
      Use WH shortcuts
    </label>

    <div class="flag-group">
      {#each ['shortest', 'secure', 'insecure'] as f}
        <button
          class="flag-btn"
          class:active={flag === f}
          on:click={() => setFlag(f)}
        >{f}</button>
      {/each}
    </div>
  </div>

  <button class="calc-btn" on:click={calculate} disabled={loading || !origin || !destination}>
    {loading ? 'Calculating…' : 'Calculate'}
  </button>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  {#if jumps.length > 0}
    <div class="result-header">
      <span class="jump-count">{totalJumps} jump{totalJumps !== 1 ? 's' : ''}</span>
      <span class="jump-breakdown">
        {gateCount} gate{gateCount !== 1 ? 's' : ''}
        {#if whCount > 0} · <span class="wh-count">{whCount} WH</span>{/if}
      </span>
    </div>
    <ul class="jump-list">
      {#each jumps as jump, i (jump.systemId)}
        <li class="jump-item" class:wh={jump.jumpType === 'wormhole'}>
          <span class="jump-index">{i + 1}</span>
          <span class="jump-name">{jump.systemName}</span>
          {#if jump.jumpType === 'wormhole'}
            <span class="jump-type-tag">WH</span>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .route-section {
    border-top: 1px solid var(--border);
    padding: 8px 0 4px;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 12px 6px;
  }

  .section-label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.15em;
    color: var(--text-secondary);
  }

  .clear-btn {
    width: 18px;
    height: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    color: var(--text-secondary);
    border-radius: 3px;
    cursor: pointer;
    transition: color 0.15s;
  }
  .clear-btn:hover { color: var(--danger); }

  .inputs {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0 8px;
  }

  .system-input {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 5px 8px;
    color: var(--text-primary);
    font-size: 12px;
    width: 100%;
    outline: none;
    transition: border-color 0.15s;
  }
  .system-input:focus { border-color: var(--accent-dim); }
  .system-input::placeholder { color: var(--text-dim); }

  .options {
    padding: 6px 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .opt-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--text-secondary);
    cursor: pointer;
  }
  .opt-label input { accent-color: var(--accent); cursor: pointer; }

  .flag-group {
    display: flex;
    gap: 3px;
  }

  .flag-btn {
    flex: 1;
    padding: 3px 0;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 3px;
    font-size: 10px;
    color: var(--text-secondary);
    cursor: pointer;
    text-transform: capitalize;
    transition: all 0.15s;
  }
  .flag-btn.active {
    background: var(--accent-dim);
    border-color: var(--accent);
    color: var(--text-primary);
  }
  .flag-btn:hover:not(.active) {
    border-color: var(--border-bright);
    color: var(--text-primary);
  }

  .calc-btn {
    margin: 0 8px 6px;
    width: calc(100% - 16px);
    padding: 6px;
    background: var(--accent-dim);
    border-radius: 4px;
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
    cursor: pointer;
    transition: background 0.15s;
  }
  .calc-btn:hover:not(:disabled) { background: var(--accent); color: #000; }
  .calc-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .error {
    margin: 0 8px 6px;
    font-size: 11px;
    color: var(--danger);
    line-height: 1.4;
  }

  .result-header {
    padding: 4px 12px 2px;
    display: flex;
    align-items: baseline;
    gap: 8px;
  }

  .jump-count {
    font-size: 11px;
    font-weight: 700;
    color: var(--accent);
  }

  .jump-breakdown {
    font-size: 10px;
    color: var(--text-secondary);
  }

  .wh-count { color: var(--accent); }

  .jump-list {
    list-style: none;
    max-height: 220px;
    overflow-y: auto;
    padding: 0 6px 4px;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .jump-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px;
    border-radius: 3px;
    font-size: 11px;
  }
  .jump-item:hover { background: var(--bg-hover); }
  .jump-item.wh { border-left: 2px solid var(--accent); padding-left: 4px; }

  .jump-index {
    min-width: 18px;
    color: var(--text-dim);
    font-size: 10px;
    text-align: right;
  }

  .jump-name {
    flex: 1;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .jump-type-tag {
    font-size: 9px;
    font-weight: 700;
    padding: 1px 4px;
    background: var(--accent-glow);
    border: 1px solid var(--accent-dim);
    border-radius: 2px;
    color: var(--accent);
    letter-spacing: 0.05em;
  }
</style>
