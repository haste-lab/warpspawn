<script lang="ts">
  import './lib/theme.css';
  import { onMount } from 'svelte';
  import { settings, projects, budget, setupState, showWizard, currentView, notifications, dismissNotification } from './lib/stores/app';
  import { getHealth, getProjects, getBudget, getSettings, connectEvents } from './lib/api';
  import { handleSSEEvent } from './lib/stores/app';
  import Dashboard from './lib/components/Dashboard.svelte';
  import SetupWizard from './lib/components/SetupWizard.svelte';
  import SettingsPanel from './lib/components/SettingsPanel.svelte';
  import SetupBanner from './lib/components/SetupBanner.svelte';
  import Header from './lib/components/Header.svelte';

  let connected = false;
  let version = 'dev';

  onMount(async () => {
    try {
      const health = await getHealth();
      connected = true;
      version = health.version;

      const [s, p, b] = await Promise.all([
        getSettings().catch(() => null),
        getProjects().catch(() => []),
        getBudget().catch(() => null),
      ]);
      if (s) settings.set(s);
      projects.set(p);
      if (b) budget.set(b);

      if ($setupState === 'unconfigured') {
        showWizard.set(true);
      }

      connectEvents(handleSSEEvent);
    } catch (e) {
      console.error('Failed to connect to backend:', e);
    }
  });

  function handleNav(view: 'dashboard' | 'settings') {
    currentView.set(view);
    showWizard.set(false);
  }
</script>

<div class="shell">
  <Header {version} {connected} on:nav={(e) => handleNav(e.detail)} on:wizard={() => showWizard.set(true)} />

  {#if $setupState === 'unconfigured' && !$showWizard}
    <SetupBanner on:setup={() => showWizard.set(true)} />
  {/if}

  {#if $notifications.length > 0}
    <div class="notifications">
      {#each $notifications as notif (notif.id)}
        <div class="notif notif-{notif.type}">
          <span>{notif.message}</span>
          {#if notif.dismissable}
            <button class="notif-dismiss" on:click={() => dismissNotification(notif.id)}>×</button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  <main class="main">
    {#if $showWizard}
      <SetupWizard on:close={() => showWizard.set(false)} on:complete={() => { showWizard.set(false); currentView.set('dashboard'); }} />
    {:else if $currentView === 'settings'}
      <SettingsPanel />
    {:else}
      <Dashboard />
    {/if}
  </main>
</div>

<style>
  .shell {
    height: 100vh;
    display: flex;
    flex-direction: column;
  }
  .main {
    flex: 1;
    overflow-y: auto;
    padding: 20px 24px;
    max-width: 1200px;
    margin: 0 auto;
    width: 100%;
  }
  .notifications {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px 24px 0;
    max-width: 1200px;
    margin: 0 auto;
    width: 100%;
  }
  .notif {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 14px;
    border-radius: var(--radius-sm);
    font-size: 0.85rem;
  }
  .notif-info { background: var(--blue-dim); color: var(--blue); }
  .notif-warning { background: var(--amber-dim); color: var(--amber); }
  .notif-error { background: var(--red-dim); color: var(--red); }
  .notif-success { background: var(--accent-dim); color: var(--green); }
  .notif-dismiss {
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    font-size: 1.1rem;
    opacity: 0.7;
  }
  .notif-dismiss:hover { opacity: 1; }
</style>
