<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import RoutePanel from './RoutePanel.svelte'
  import type { app } from '../../wailsjs/go/models'

  type Character = app.Character
  type RouteJump = app.RouteJump

  export let characters: Character[] = []
  export let adding: boolean = false

  const dispatch = createEventDispatcher()
</script>

<aside>
  <section>
    <div class="section-header">
      <span class="section-label">CHARACTERS</span>
      <button
        class="add-btn"
        on:click={() => dispatch('add')}
        disabled={adding}
        title="Add character"
      >
        {adding ? '…' : '+'}
      </button>
    </div>

    <ul class="char-list">
      {#each characters as char (char.id)}
        <li class="char-card">
          <div class="char-info">
            <span class="char-name">{char.name}</span>
            <span class="char-system">{char.currentSystem || 'Locating…'}</span>
          </div>
          <button
            class="remove-btn"
            on:click={() => dispatch('remove', char.id)}
            title="Remove character"
          >×</button>
        </li>
      {:else}
        <li class="empty-hint">No characters added.<br/>Press + to authorise one.</li>
      {/each}
    </ul>
  </section>

  <RoutePanel on:route={(e) => dispatch('route', e.detail)} />
</aside>

<style>
  aside {
    background: var(--bg-panel);
    border-right: 1px solid var(--border);
    overflow-y: auto;
    padding: 8px 0;
    display: flex;
    flex-direction: column;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 12px;
    margin-bottom: 4px;
  }

  .section-label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.15em;
    color: var(--text-secondary);
  }

  .add-btn {
    width: 20px;
    height: 20px;
    border-radius: 3px;
    background: var(--accent-dim);
    color: var(--bg-base);
    font-size: 16px;
    line-height: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s;
  }
  .add-btn:hover:not(:disabled) { background: var(--accent); }
  .add-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .char-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0 6px;
  }

  .char-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 8px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 4px;
    transition: border-color 0.15s, background 0.15s;
  }
  .char-card:hover { background: var(--bg-hover); border-color: var(--border-bright); }

  .char-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }

  .char-name {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .char-system { font-size: 10px; color: var(--text-secondary); }

  .remove-btn {
    flex-shrink: 0;
    width: 18px;
    height: 18px;
    border-radius: 3px;
    font-size: 14px;
    color: var(--text-dim);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.15s, background 0.15s;
  }
  .remove-btn:hover { color: var(--danger); background: rgba(208, 64, 64, 0.1); }

  .empty-hint {
    padding: 16px 12px;
    font-size: 11px;
    color: var(--text-secondary);
    text-align: center;
    line-height: 1.6;
  }
</style>
