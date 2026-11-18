<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte'
  import { ImportBookmarks } from '../../wailsjs/go/app/App'
  import type { app } from '../../wailsjs/go/models'

  const dispatch = createEventDispatcher<{ close: void }>()

  let text = ''
  let importing = false
  let error = ''
  let result: app.BookmarkImportResult | null = null
  let textarea: HTMLTextAreaElement

  onMount(() => textarea?.focus())

  async function runImport() {
    if (!text.trim()) return
    importing = true
    error = ''
    result = null
    try {
      result = await ImportBookmarks(text)
      if (!result.skipped?.length && !result.warnings?.length) text = ''
    } catch (e: any) {
      error = e?.toString() ?? 'Import failed'
    } finally {
      importing = false
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close')
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) runImport()
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="backdrop" on:click|self={() => dispatch('close')} on:mousedown|stopPropagation on:wheel|stopPropagation>
  <div class="panel" role="dialog" aria-label="Import bookmarks" on:keydown={onKey}>
    <div class="panel-header">
      <span class="panel-title">IMPORT BOOKMARKS</span>
      <button class="close-btn" on:click={() => dispatch('close')} title="Close">×</button>
    </div>

    <p class="hint">
      Select bookmarks in People &amp; Places, Ctrl+C, paste below.
      Labels must look like <code>PREFIX-SIG-Destination</code>; use
      <code>[note]</code> as the destination when the far side is unknown.
      Prefixes: <code>P2P</code>/<code>PP</code> Pochven→Pochven · <code>P</code> k-space→Pochven ·
      <code>H</code>/<code>L</code>/<code>N</code> → high/low/null · <code>W</code> → J-space.
    </p>

    <textarea
      bind:this={textarea}
      bind:value={text}
      placeholder="P2P-ORE-Krirald	Coordinate	12	Ala	Krai Veles	Pochven	…"
      spellcheck="false"
    ></textarea>

    <div class="actions">
      <span class="shortcut">Ctrl+Enter</span>
      <button class="import-btn" on:click={runImport} disabled={importing || !text.trim()}>
        {importing ? 'Importing…' : 'Import'}
      </button>
    </div>

    {#if error}
      <div class="error">{error}</div>
    {/if}

    {#if result}
      <div class="summary">
        <span><b>{result.added}</b> new</span>
        <span><b>{result.updated}</b> updated</span>
        <span><b>{result.systems}</b> systems added</span>
        <span><b>{result.signatures}</b> unknown-dest sigs</span>
      </div>
      {#if result.warnings?.length}
        <ul class="skipped warnings">
          {#each result.warnings as w}
            <li>{w}</li>
          {/each}
        </ul>
      {/if}
      {#if result.skipped?.length}
        <ul class="skipped">
          {#each result.skipped as s}
            <li>{s}</li>
          {/each}
        </ul>
      {/if}
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 20;
  }

  .panel {
    width: min(640px, calc(100% - 32px));
    background: var(--bg-panel);
    border: 1px solid var(--border-bright);
    border-radius: 6px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .panel-header { display: flex; align-items: center; justify-content: space-between; }

  .panel-title {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.15em;
    color: var(--text-secondary);
  }

  .close-btn {
    width: 20px; height: 20px;
    font-size: 16px;
    color: var(--text-dim);
    border-radius: 3px;
  }
  .close-btn:hover { color: var(--text-primary); }

  .hint { font-size: 11px; color: var(--text-secondary); line-height: 1.5; }
  .hint code { color: var(--accent); font-size: 11px; }

  textarea {
    height: 180px;
    resize: vertical;
    background: var(--bg-base);
    color: var(--text-primary);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 8px;
    font-family: monospace;
    font-size: 11px;
    white-space: pre;
    tab-size: 4;
  }
  textarea:focus { outline: none; border-color: var(--accent-dim); }

  .actions { display: flex; align-items: center; justify-content: flex-end; gap: 10px; }
  .shortcut { font-size: 10px; color: var(--text-dim); }

  .import-btn {
    padding: 5px 14px;
    border-radius: 4px;
    background: var(--accent-dim);
    color: var(--bg-base);
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.05em;
  }
  .import-btn:hover:not(:disabled) { background: var(--accent); }
  .import-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .error { font-size: 11px; color: var(--danger); }

  .summary {
    display: flex;
    flex-wrap: wrap;
    gap: 14px;
    font-size: 11px;
    color: var(--text-secondary);
  }
  .summary b { color: var(--text-primary); }

  .skipped {
    list-style: none;
    max-height: 100px;
    overflow-y: auto;
    font-size: 10px;
    color: var(--text-dim);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .skipped li::before { content: 'skipped · '; color: var(--danger); }
  .warnings li::before { content: 'check · '; color: #ddaa22; }
</style>
