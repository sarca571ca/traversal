<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte'
  import { GetSignatures, SaveSignature, DeleteSignature, ClearSignatures, GetSystemStatics } from '../../wailsjs/go/app/App'
  import type { app } from '../../wailsjs/go/models'

  export let systemID: number
  export let systemName: string = ''
  export let systemSecurity: number = 0

  const dispatch = createEventDispatcher<{ close: void }>()

  const isJspace = systemID >= 31000000 && systemID < 32000000

  // Maps well-known static wormhole type codes to their destination.
  const WH_DEST: Record<string, string> = {
    'K162': 'inbound',
    'B274': 'Hi-Sec', 'X702': 'Hi-Sec', 'M555': 'Hi-Sec', 'A641': 'Hi-Sec',
    'N062': 'Lo-Sec', 'D382': 'Lo-Sec', 'E587': 'Lo-Sec',
    'Z060': 'Null-Sec', 'O477': 'Null-Sec', 'U574': 'Null-Sec',
    'T458': 'Pochven', 'F055': 'Pochven',
    'D845': 'C1', 'Z971': 'C1', 'R051': 'C1', 'P060': 'C1',
    'O128': 'C2', 'H121': 'C2', 'N766': 'C2',
    'Y683': 'C3', 'C125': 'C3', 'U210': 'C3',
    'X877': 'C4', 'Z647': 'C4', 'O883': 'C4', 'C008': 'C4',
    'H900': 'C5', 'V753': 'C5',
    'Z142': 'C6', 'B041': 'C6',
  }

  function whClass(sec: number): string {
    if (sec <= -0.99) return 'Thera'
    if (sec <= -0.95) return 'Shattered'
    if (Math.abs(sec + 0.9) < 0.05) return 'C1'
    if (Math.abs(sec + 0.8) < 0.05) return 'C2'
    if (Math.abs(sec + 0.7) < 0.05) return 'C3'
    if (Math.abs(sec + 0.6) < 0.05) return 'C4'
    if (Math.abs(sec + 0.5) < 0.05) return 'C5'
    if (Math.abs(sec + 0.4) < 0.05) return 'C6'
    return 'J-Space'
  }

  let statics: string[] = []
  let staticsLoading = false

  onMount(async () => {
    if (isJspace) {
      staticsLoading = true
      statics = (await GetSystemStatics(systemID)) ?? []
      staticsLoading = false
    }
  })

  const SIG_TYPES = ['Unknown', 'Wormhole', 'Data', 'Relic', 'Gas', 'Combat']

  type SigEntry = app.SigEntry

  let sigs: SigEntry[] = []
  let newSigID = ''
  let newSigType = 'Unknown'
  let newNotes = ''
  let adding = false
  let error = ''

  async function reload() {
    sigs = (await GetSignatures(systemID)) ?? []
  }
  reload()

  async function addSig() {
    if (!newSigID.trim()) return
    adding = true
    error = ''
    try {
      await SaveSignature(systemID, newSigID.trim().toUpperCase(), newSigType, newNotes.trim())
      newSigID = ''
      newNotes = ''
      await reload()
    } catch (e: any) {
      error = e?.toString() ?? 'Save failed'
    } finally {
      adding = false
    }
  }

  async function remove(id: number) {
    try {
      await DeleteSignature(id)
      await reload()
    } catch (e: any) {
      error = e?.toString() ?? 'Delete failed'
    }
  }

  async function clearAll() {
    if (!confirm(`Clear all signatures for ${systemName}?`)) return
    try {
      await ClearSignatures(systemID)
      await reload()
    } catch (e: any) {
      error = e?.toString() ?? 'Clear failed'
    }
  }

  function typeColor(t: string): string {
    switch (t.toLowerCase()) {
      case 'wormhole': return 'var(--accent)'
      case 'data':     return '#4fc3f7'
      case 'relic':    return '#ce93d8'
      case 'gas':      return '#a5d6a7'
      case 'combat':   return 'var(--danger)'
      default:         return 'var(--text-secondary)'
    }
  }
</script>

<div class="sig-panel" role="dialog" aria-label="Signatures">
  <div class="panel-header">
    <div class="panel-title-row">
      <span class="panel-title">SIGNATURES</span>
      <span class="sys-name">{systemName || systemID}</span>
    </div>
    <button class="close-btn" on:click={() => dispatch('close')} title="Close">×</button>
  </div>

  {#if isJspace}
    <div class="jspace-info">
      <span class="jspace-class">{whClass(systemSecurity)}</span>
      <span class="jspace-label">statics:</span>
      {#if staticsLoading}
        <span class="jspace-dim">…</span>
      {:else if statics.length === 0}
        <span class="jspace-dim">none found</span>
      {:else}
        {#each statics as code}
          <span class="jspace-static" title={WH_DEST[code] ?? ''}>
            {code}{WH_DEST[code] ? ` → ${WH_DEST[code]}` : ''}
          </span>
        {/each}
      {/if}
    </div>
  {/if}

  <div class="sig-list">
    {#if sigs.length === 0}
      <p class="empty">No signatures logged.</p>
    {:else}
      {#each sigs as sig (sig.id)}
        <div class="sig-row">
          <span class="sig-id">{sig.sigId}</span>
          <span class="sig-type" style:color={typeColor(sig.sigType)}>{sig.sigType || '?'}</span>
          {#if sig.notes}
            <span class="sig-notes" title={sig.notes}>{sig.notes}</span>
          {/if}
          <button class="del-btn" on:click={() => remove(sig.id)} title="Remove">×</button>
        </div>
      {/each}
    {/if}
  </div>

  <div class="add-form">
    <input
      class="sig-input"
      placeholder="ABC-123"
      bind:value={newSigID}
      on:keydown={(e) => e.key === 'Enter' && addSig()}
      maxlength="8"
    />
    <select class="sig-select" bind:value={newSigType}>
      {#each SIG_TYPES as t}
        <option value={t}>{t}</option>
      {/each}
    </select>
    <button class="add-btn" on:click={addSig} disabled={adding || !newSigID.trim()}>+</button>
  </div>

  <input
    class="notes-input"
    placeholder="Notes (optional)"
    bind:value={newNotes}
    on:keydown={(e) => e.key === 'Enter' && addSig()}
  />

  {#if error}
    <p class="error">{error}</p>
  {/if}

  {#if sigs.length > 0}
    <button class="clear-btn" on:click={clearAll}>Clear All</button>
  {/if}
</div>

<style>
  .sig-panel {
    position: absolute;
    top: 12px;
    left: 12px;
    width: 240px;
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

  .panel-title-row {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }

  .panel-title {
    font-size: 9px;
    font-weight: 700;
    letter-spacing: 0.15em;
    color: var(--accent);
  }

  .sys-name {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .close-btn {
    flex-shrink: 0;
    width: 18px; height: 18px;
    display: flex; align-items: center; justify-content: center;
    font-size: 16px; color: var(--text-secondary);
    border-radius: 3px; cursor: pointer; transition: color 0.15s;
  }
  .close-btn:hover { color: var(--text-primary); }

  .sig-list {
    max-height: 180px;
    overflow-y: auto;
    padding: 6px 8px;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .empty {
    font-size: 11px;
    color: var(--text-dim);
    text-align: center;
    padding: 8px 0;
  }

  .sig-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px 4px;
    border-radius: 3px;
  }
  .sig-row:hover { background: var(--bg-hover); }

  .sig-id {
    font-size: 11px;
    font-family: monospace;
    font-weight: 700;
    color: var(--text-primary);
    min-width: 56px;
  }

  .sig-type {
    font-size: 10px;
    font-weight: 600;
    flex: 1;
  }

  .sig-notes {
    font-size: 10px;
    color: var(--text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 60px;
  }

  .del-btn {
    flex-shrink: 0;
    width: 16px; height: 16px;
    font-size: 13px;
    color: var(--text-dim);
    display: flex; align-items: center; justify-content: center;
    border-radius: 2px; cursor: pointer; transition: color 0.15s;
  }
  .del-btn:hover { color: var(--danger); }

  .add-form {
    display: flex;
    gap: 4px;
    padding: 6px 8px 0;
  }

  .sig-input {
    flex: 1;
    min-width: 0;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 4px 6px;
    font-size: 12px;
    font-family: monospace;
    color: var(--text-primary);
    text-transform: uppercase;
    outline: none;
  }
  .sig-input:focus { border-color: var(--accent-dim); }

  .sig-select {
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 4px 4px;
    font-size: 11px;
    color: var(--text-primary);
    outline: none;
    cursor: pointer;
  }
  .sig-select:focus { border-color: var(--accent-dim); }
  .sig-select option { background: var(--bg-panel); }

  .add-btn {
    flex-shrink: 0;
    width: 26px;
    background: var(--accent-dim);
    border-radius: 3px;
    font-size: 16px;
    color: var(--text-primary);
    cursor: pointer;
    transition: background 0.15s;
    display: flex; align-items: center; justify-content: center;
  }
  .add-btn:hover:not(:disabled) { background: var(--accent); color: #000; }
  .add-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .notes-input {
    margin: 4px 8px 6px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 4px 6px;
    font-size: 11px;
    color: var(--text-primary);
    outline: none;
    width: calc(100% - 16px);
  }
  .notes-input:focus { border-color: var(--accent-dim); }
  .notes-input::placeholder { color: var(--text-dim); }

  .error { margin: 0 8px 4px; font-size: 11px; color: var(--danger); }

  .clear-btn {
    margin: 0 8px 8px;
    padding: 4px;
    background: transparent;
    border: 1px solid rgba(208,64,64,0.25);
    border-radius: 3px;
    font-size: 10px;
    color: var(--danger);
    cursor: pointer;
    opacity: 0.7;
    transition: opacity 0.15s;
    width: calc(100% - 16px);
  }
  .clear-btn:hover { opacity: 1; }

  .jspace-info {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 5px;
    padding: 5px 10px;
    background: rgba(64, 200, 128, 0.05);
    border-bottom: 1px solid var(--border);
    font-size: 11px;
  }
  .jspace-class {
    font-weight: 700;
    color: #40c880;
    min-width: 28px;
  }
  .jspace-label {
    color: var(--text-secondary);
    font-size: 10px;
  }
  .jspace-static {
    font-family: monospace;
    font-size: 10px;
    color: var(--text-primary);
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 1px 5px;
  }
  .jspace-dim {
    font-size: 10px;
    color: var(--text-dim);
    font-style: italic;
  }
</style>
