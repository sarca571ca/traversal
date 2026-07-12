<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'

  type Toast = { id: number; message: string; timer: ReturnType<typeof setTimeout> }

  let toasts: Toast[] = []
  let nextID = 0
  let unsubscribe: (() => void) | null = null

  function addToasts(messages: string[]) {
    for (const message of messages) {
      const id = nextID++
      const timer = setTimeout(() => dismiss(id), 6000)
      toasts = [...toasts, { id, message, timer }]
    }
  }

  function dismiss(id: number) {
    const t = toasts.find(t => t.id === id)
    if (t) clearTimeout(t.timer)
    toasts = toasts.filter(t => t.id !== id)
  }

  onMount(() => {
    unsubscribe = EventsOn('wh:expired', (messages: string[]) => {
      addToasts(messages.map(m => `WH expired: ${m}`))
    })
  })

  onDestroy(() => {
    if (unsubscribe) unsubscribe()
    toasts.forEach(t => clearTimeout(t.timer))
  })
</script>

<div class="toast-container">
  {#each toasts as toast (toast.id)}
    <div class="toast" role="alert">
      <span class="toast-icon">◌</span>
      <span class="toast-msg">{toast.message}</span>
      <button class="toast-close" on:click={() => dismiss(toast.id)}>×</button>
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    bottom: 20px;
    right: 20px;
    display: flex;
    flex-direction: column-reverse;
    gap: 6px;
    z-index: 100;
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-panel);
    border: 1px solid rgba(208, 64, 64, 0.4);
    border-radius: 5px;
    box-shadow: 0 4px 16px rgba(0,0,0,0.5);
    font-size: 12px;
    color: var(--text-primary);
    pointer-events: all;
    animation: slideIn 0.2s ease-out;
    max-width: 280px;
  }

  .toast-icon { color: var(--danger); flex-shrink: 0; }

  .toast-msg { flex: 1; line-height: 1.3; }

  .toast-close {
    flex-shrink: 0;
    width: 16px; height: 16px;
    display: flex; align-items: center; justify-content: center;
    font-size: 14px; color: var(--text-secondary);
    cursor: pointer; border-radius: 2px; transition: color 0.15s;
  }
  .toast-close:hover { color: var(--text-primary); }

  @keyframes slideIn {
    from { opacity: 0; transform: translateY(8px); }
    to   { opacity: 1; transform: translateY(0); }
  }
</style>
