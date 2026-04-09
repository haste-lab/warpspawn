<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { budget, setupState, currentView } from '../stores/app';

  export let version = 'dev';
  export let connected = false;

  const dispatch = createEventDispatcher();

  $: budgetPct = $budget ? Math.min(100, ($budget.daily_cost_usd / $budget.daily_limit_usd) * 100) : 0;
  $: budgetColor = budgetPct > 80 ? 'var(--red)' : budgetPct > 50 ? 'var(--amber)' : 'var(--accent)';
</script>

<header class="header">
  <div class="header-left">
    <button class="logo" on:click={() => dispatch('nav', 'dashboard')}>
      <span class="logo-icon">⚡</span>
      <span class="logo-text">Warpspawn</span>
      <span class="version">{version}</span>
    </button>

    <nav class="nav">
      <button class="nav-btn" class:active={$currentView === 'dashboard'} on:click={() => dispatch('nav', 'dashboard')}>
        Dashboard
      </button>
      <button class="nav-btn" class:active={$currentView === 'settings'} on:click={() => dispatch('nav', 'settings')}>
        Settings
      </button>
      <button class="nav-btn" class:active={$currentView === 'help'} on:click={() => dispatch('nav', 'help')}>
        Help
      </button>
    </nav>
  </div>

  <div class="header-right">
    {#if $budget}
      <div class="budget-pill" title="Daily token budget: ${$budget.daily_cost_usd.toFixed(2)} / ${$budget.daily_limit_usd.toFixed(2)}">
        <div class="budget-bar">
          <div class="budget-fill" style="width: {budgetPct}%; background: {budgetColor}"></div>
        </div>
        <span class="budget-text">${$budget.daily_cost_usd.toFixed(2)}</span>
      </div>
    {/if}

    {#if $setupState === 'unconfigured'}
      <button class="btn btn-primary btn-sm" on:click={() => dispatch('wizard')}>
        Setup
      </button>
    {/if}

    <div class="status-dot" class:connected title={connected ? 'Connected' : 'Disconnected'}></div>
  </div>
</header>

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 24px;
    height: 48px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-surface);
    flex-shrink: 0;
  }
  .header-left, .header-right {
    display: flex;
    align-items: center;
    gap: 16px;
  }
  .logo {
    display: flex;
    align-items: center;
    gap: 6px;
    background: none;
    border: none;
    color: var(--text);
    cursor: pointer;
    font-size: 0.95rem;
    font-weight: 600;
  }
  .logo-icon { font-size: 1.1rem; }
  .version {
    font-size: 0.7rem;
    color: var(--text-dim);
    font-weight: 400;
  }
  .nav {
    display: flex;
    gap: 2px;
  }
  .nav-btn {
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 4px 10px;
    border-radius: var(--radius-sm);
    font-size: 0.85rem;
  }
  .nav-btn:hover { color: var(--text); background: var(--bg-hover); }
  .nav-btn.active { color: var(--text); background: var(--bg-elevated); }
  .budget-pill {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 10px;
    background: var(--bg-elevated);
    border-radius: 999px;
    font-size: 0.75rem;
  }
  .budget-bar {
    width: 48px;
    height: 4px;
    background: var(--bg);
    border-radius: 999px;
    overflow: hidden;
  }
  .budget-fill {
    height: 100%;
    border-radius: 999px;
    transition: width 0.3s;
  }
  .budget-text {
    color: var(--text-muted);
    font-family: var(--font-mono);
  }
  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--red);
  }
  .status-dot.connected { background: var(--green); }
</style>
