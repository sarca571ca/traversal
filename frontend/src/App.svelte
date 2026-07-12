<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Map from './components/Map.svelte'
  import Toast from './components/Toast.svelte'
  import { GetCharacters, AddCharacter, RemoveCharacter } from '../wailsjs/go/app/App'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import type { app } from '../wailsjs/go/models'

  type Character = app.Character
  type RouteJump = app.RouteJump

  let characters: Character[] = []
  let routeJumps: RouteJump[] | null = null
  let adding = false

  async function load() {
    characters = await GetCharacters()
  }

  async function handleAdd() {
    adding = true
    try {
      await AddCharacter()
      await load()
    } finally {
      adding = false
    }
  }

  async function handleRemove(id: number) {
    await RemoveCharacter(id)
    await load()
  }

  let unsubscribe: (() => void) | null = null
  onMount(() => {
    load()
    // Re-fetch characters on every map update so the sidebar shows live locations.
    unsubscribe = EventsOn('map:update', load)
  })
  onDestroy(() => { if (unsubscribe) unsubscribe() })
</script>

<header>
  <span class="logo">TRAVERSAL</span>
  <span class="status">● {characters.length} character{characters.length !== 1 ? 's' : ''}</span>
</header>

<Sidebar
  {characters}
  {adding}
  on:add={handleAdd}
  on:remove={(e) => handleRemove(e.detail)}
  on:route={(e) => routeJumps = e.detail}
/>

<Map {routeJumps} />

<Toast />

<style>
  header {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 16px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
  }

  .logo {
    font-size: 14px;
    font-weight: 700;
    letter-spacing: 0.2em;
    color: var(--accent);
  }

  .status {
    font-size: 11px;
    color: var(--text-secondary);
    letter-spacing: 0.05em;
  }
</style>
