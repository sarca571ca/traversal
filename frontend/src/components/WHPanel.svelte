<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte'
  import { UpdateConnection, DeleteConnection } from '../../wailsjs/go/app/App'
  import type { app } from '../../wailsjs/go/models'

  export let conn: app.MapConnection
  export let nameA: string = ''
  export let nameB: string = ''

  const dispatch = createEventDispatcher<{ close: void; saved: void; deleted: void }>()

  const SHIP_SIZES = [
    { value: '',       label: '— unknown —' },
    { value: 'S',      label: 'Small  (Frigate / Destroyer)' },
    { value: 'M',      label: 'Medium  (Cruiser / Battlecruiser)' },
    { value: 'L',      label: 'Large  (Battleship)' },
    { value: 'XL',     label: 'X-Large  (Capital)' },
  ]
  const MASS_OPTS = [
    { value: 'stable',   label: 'Stable (>50%)' },
    { value: 'half',     label: 'Below 50%' },
    { value: 'critical', label: 'Critical' },
  ]
  const TIMERS = [
    { hours: -1,  label: 'Keep' },
    { hours: 24,  label: '24h' },
    { hours: 4,   label: '4h' },
    { hours: 1,   label: '1h' },
    { hours: 0,   label: 'None' },
  ]

  let whTypeA   = conn.whTypeA   ?? ''
  let whTypeB   = conn.whTypeB   ?? ''
  let sigIdA    = conn.sigIdA    ?? ''
  let sigIdB    = conn.sigIdB    ?? ''
  let shipSize  = conn.shipSize  ?? ''
  let massStatus = conn.massStatus ?? 'stable'
  let eol       = conn.eol       ?? false
  let timerHours = -1  // -1 = keep existing
  let saving    = false
  let error     = ''

  // Reactive remaining-time display, updated every second.
  let remaining = ''
  function calcRemaining() {
    if (!conn.expiresAt) { remaining = ''; return }
    const secs = conn.expiresAt - Math.floor(Date.now() / 1000)
    if (secs <= 0) { remaining = 'Expired'; return }
    const h = Math.floor(secs / 3600)
    const m = Math.floor((secs % 3600) / 60)
    const s = secs % 60
    remaining = h > 0 ? `${h}h ${m}m` : m > 0 ? `${m}m ${s}s` : `${s}s`
  }
  calcRemaining()
  const tick = setInterval(calcRemaining, 1000)
  onDestroy(() => clearInterval(tick))

  async function save() {
    saving = true
    error = ''
    try {
      await UpdateConnection(conn.id, whTypeA, whTypeB, sigIdA, sigIdB, shipSize, massStatus, eol, timerHours)
      dispatch('saved')
    } catch (e: any) {
      error = e?.toString() ?? 'Save failed'
    } finally {
      saving = false
    }
  }

  async function remove() {
    saving = true
    error = ''
    try {
      await DeleteConnection(conn.id)
      dispatch('deleted')
    } catch (e: any) {
      error = e?.toString() ?? 'Delete failed'
    } finally {
      saving = false
    }
  }
</script>

<div class="wh-panel" role="dialog" aria-label="Wormhole info">
  <div class="panel-header">
    <span class="panel-title">WORMHOLE</span>
    <button class="close-btn" on:click={() => dispatch('close')} title="Close">×</button>
  </div>

  <!-- Two-sided sig/type fields -->
  <div class="sides">
    <div class="side">
      <span class="side-label">{nameA || conn.systemA}</span>
      <input class="field-input" placeholder="Sig ID" bind:value={sigIdA} on:keydown={(e) => e.key === 'Enter' && save()} />
      <input class="field-input" placeholder="WH type" bind:value={whTypeA} on:keydown={(e) => e.key === 'Enter' && save()} />
    </div>
    <div class="side-arrow">⟷</div>
    <div class="side">
      <span class="side-label">{nameB || conn.systemB}</span>
      <input class="field-input" placeholder="Sig ID" bind:value={sigIdB} on:keydown={(e) => e.key === 'Enter' && save()} />
      <input class="field-input" placeholder="WH type" bind:value={whTypeB} on:keydown={(e) => e.key === 'Enter' && save()} />
    </div>
  </div>

  <div class="fields">
    <label class="field">
      <span class="field-label">SHIP SIZE</span>
      <select class="field-select" bind:value={shipSize}>
        {#each SHIP_SIZES as s}
          <option value={s.value}>{s.label}</option>
        {/each}
      </select>
    </label>

    <div class="field">
      <span class="field-label">MASS STATUS</span>
      <div class="radio-group">
        {#each MASS_OPTS as opt}
          <label class="radio-label" class:active={massStatus === opt.value}>
            <input type="radio" bind:group={massStatus} value={opt.value} />
            {opt.label}
          </label>
        {/each}
      </div>
    </div>

    <label class="field eol-field">
      <input type="checkbox" bind:checked={eol} />
      <span class="eol-text" class:active={eol}>End of Life</span>
    </label>

    <div class="field">
      <span class="field-label">
        TIMER
        {#if remaining}<span class="remaining" class:expired={remaining === 'Expired'}> — {remaining}</span>{/if}
      </span>
      <div class="timer-group">
        {#each TIMERS as t}
          <button
            class="timer-btn"
            class:active={timerHours === t.hours}
            on:click={() => timerHours = t.hours}
          >{t.label}</button>
        {/each}
      </div>
    </div>
  </div>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  <div class="actions">
    <button class="delete-btn" on:click={remove} disabled={saving}>Delete</button>
    <button class="save-btn" on:click={save} disabled={saving}>
      {saving ? 'Saving…' : 'Save'}
    </button>
  </div>
</div>

<style>
  .wh-panel {
    position: absolute;
    top: 12px;
    right: 12px;
    width: 280px;
    background: var(--bg-panel);
    border: 1px solid var(--border-bright);
    border-radius: 6px;
    box-shadow: 0 4px 24px rgba(0,0,0,0.5);
    z-index: 10;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px 6px;
    background: var(--bg-card);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .panel-title {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.15em;
    color: var(--accent);
  }

  .close-btn {
    width: 18px; height: 18px;
    display: flex; align-items: center; justify-content: center;
    font-size: 16px; color: var(--text-secondary);
    border-radius: 3px; cursor: pointer; transition: color 0.15s;
  }
  .close-btn:hover { color: var(--text-primary); }

  /* Two-sided layout */
  .sides {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    gap: 6px;
    align-items: start;
    padding: 10px 10px 6px;
    border-bottom: 1px solid var(--border);
  }

  .side {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }

  .side-label {
    font-size: 10px;
    font-weight: 600;
    color: var(--accent);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: center;
  }

  .side-arrow {
    font-size: 14px;
    color: var(--text-dim);
    padding-top: 18px;
  }

  .fields {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .field-label {
    font-size: 9px;
    font-weight: 700;
    letter-spacing: 0.12em;
    color: var(--text-secondary);
  }

  .field-input, .field-select {
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 4px 7px;
    font-size: 12px;
    color: var(--text-primary);
    outline: none;
    transition: border-color 0.15s;
    width: 100%;
  }
  .field-input:focus, .field-select:focus { border-color: var(--accent-dim); }
  .field-input::placeholder { color: var(--text-dim); }
  .field-select option { background: var(--bg-panel); }

  .radio-group { display: flex; flex-direction: column; gap: 3px; }

  .radio-label {
    display: flex; align-items: center; gap: 6px;
    font-size: 11px; color: var(--text-secondary);
    cursor: pointer; padding: 3px 6px; border-radius: 3px;
    transition: background 0.1s;
  }
  .radio-label:hover { background: var(--bg-hover); }
  .radio-label.active { color: var(--text-primary); }
  .radio-label input { accent-color: var(--accent); }

  .eol-field { flex-direction: row; align-items: center; gap: 8px; cursor: pointer; }
  .eol-field input { accent-color: var(--danger); }
  .eol-text { font-size: 11px; color: var(--text-secondary); transition: color 0.15s; }
  .eol-text.active { color: var(--danger); font-weight: 600; }

  .remaining { color: var(--accent); font-weight: 600; }
  .remaining.expired { color: var(--danger); }

  .timer-group { display: flex; gap: 4px; }

  .timer-btn {
    flex: 1;
    padding: 4px 0;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 3px;
    font-size: 11px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s;
  }
  .timer-btn.active {
    background: var(--accent-dim);
    border-color: var(--accent);
    color: var(--text-primary);
  }
  .timer-btn:hover:not(.active) {
    border-color: var(--border-bright);
    color: var(--text-primary);
  }

  .error { margin: 0 10px; font-size: 11px; color: var(--danger); }

  .actions {
    display: flex; gap: 6px;
    padding: 8px 10px;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
  }

  .save-btn {
    flex: 1; padding: 5px;
    background: var(--accent-dim); border-radius: 3px;
    font-size: 12px; font-weight: 600; color: var(--text-primary);
    cursor: pointer; transition: background 0.15s;
  }
  .save-btn:hover:not(:disabled) { background: var(--accent); color: #000; }
  .save-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .delete-btn {
    padding: 5px 10px;
    background: rgba(208, 64, 64, 0.15);
    border: 1px solid rgba(208, 64, 64, 0.3);
    border-radius: 3px; font-size: 12px; color: var(--danger);
    cursor: pointer; transition: background 0.15s;
  }
  .delete-btn:hover:not(:disabled) { background: rgba(208, 64, 64, 0.3); }
  .delete-btn:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
