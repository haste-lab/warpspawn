<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { projects, canAct, settings, addNotification, selectedProjectId, refreshTrigger } from '../stores/app';
  import { createProject, getProjects } from '../api';
  import ProjectCard from './ProjectCard.svelte';
  import ProjectDetail from './ProjectDetail.svelte';

  // Auto-refresh project list when state changes
  let dashRefreshTimer: ReturnType<typeof setTimeout> | null = null;
  const unsubRefresh = refreshTrigger.subscribe(async () => {
    if (dashRefreshTimer) clearTimeout(dashRefreshTimer);
    dashRefreshTimer = setTimeout(async () => {
      try {
        const updated = await getProjects();
        projects.set(updated);
      } catch { /* ignore */ }
    }, 1000);
  });
  onDestroy(() => { unsubRefresh(); if (dashRefreshTimer) clearTimeout(dashRefreshTimer); });

  interface ModelOption {
    provider: string;
    id: string;
    name: string;
  }

  let showNewProject = false;
  let newProjectBrief = '';
  let newProjectName = '';
  let creating = false;

  let modelStrategy: 'defaults' | 'custom' = 'defaults';
  let availableModels: ModelOption[] = [];
  let customRoles: Record<string, { provider: string; model: string }> = {};

  // Load available models when the form opens
  async function loadModels() {
    try {
      const resp = await fetch('/api/models', {
        headers: { 'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}` }
      });
      if (resp.ok) {
        availableModels = await resp.json();
      }
    } catch { /* ignore */ }
  }

  function openNewProject() {
    showNewProject = true;
    modelStrategy = 'defaults';
    // Initialize custom roles from current defaults
    if ($settings?.roles) {
      customRoles = {};
      for (const [role, cfg] of Object.entries($settings.roles)) {
        customRoles[role] = { provider: cfg.provider, model: cfg.model };
      }
    }
    loadModels();
  }

  function handleCustomRoleChange(role: string, value: string) {
    const slashIdx = value.indexOf('/');
    if (slashIdx < 0) return;
    customRoles[role] = {
      provider: value.substring(0, slashIdx),
      model: value.substring(slashIdx + 1),
    };
    customRoles = customRoles; // trigger reactivity
  }

  const roleLabels: Record<string, { label: string; hint: string }> = {
    'mission-control': { label: '🎛️ Mission Control', hint: 'Planning & orchestration' },
    'architect':       { label: '🏗️ Architect', hint: 'System design' },
    'ux':              { label: '✏️ UX Designer', hint: 'User journeys' },
    'builder':         { label: '🛠️ Builder', hint: 'Code generation' },
    'builder-light':   { label: '🔧 Builder Light', hint: 'Simple tasks' },
    'reviewer-qa':     { label: '✅ Reviewer / QA', hint: 'Validation' },
  };

  async function handleCreate() {
    if (!newProjectBrief.trim()) return;
    creating = true;
    try {
      const result = await createProject(
        newProjectBrief.trim(),
        newProjectName.trim() || undefined,
        modelStrategy,
        modelStrategy === 'custom' ? customRoles : undefined,
      );
      addNotification('success', `Project "${result.id}" created`);

      // Refresh project list
      const updated = await getProjects();
      projects.set(updated);

      // Open the new project
      selectedProjectId.set(result.id);

      showNewProject = false;
      newProjectBrief = '';
      newProjectName = '';
      modelStrategy = 'defaults';
    } catch (e: any) {
      addNotification('error', `Failed to create project: ${e.message}`);
    } finally {
      creating = false;
    }
  }

  function openProject(id: string) {
    selectedProjectId.set(id);
  }
</script>

<div class="dashboard">
  {#if $selectedProjectId}
    <ProjectDetail projectId={$selectedProjectId} on:back={() => selectedProjectId.set(null)} />
  {:else}
    <div class="dashboard-header">
      <h1>Projects</h1>
      <button class="btn btn-primary" on:click={openNewProject} disabled={!$canAct}>
        + New Project
      </button>
    </div>

    {#if showNewProject}
      <div class="card new-project-form">
        <h3>Create a new project</h3>
        <p class="text-muted text-sm mb-2">Describe what you want to build. Warpspawn will decompose it into tasks and start building.</p>

        <div class="flex-col gap-3">
          <div>
            <label>Project name (optional)</label>
            <input type="text" bind:value={newProjectName} placeholder="e.g., Weather Dashboard" />
          </div>
          <div>
            <label>Project brief</label>
            <textarea bind:value={newProjectBrief} rows="4"
              placeholder="Build a local weather dashboard that fetches data from Open-Meteo API and displays current conditions and 5-day forecast. Use vanilla HTML/CSS/JS. No framework."></textarea>
          </div>

          <div class="strategy-section">
            <label>Model strategy</label>
            <div class="strategy-options">
              <button
                class="strategy-btn"
                class:active={modelStrategy === 'defaults'}
                on:click={() => modelStrategy = 'defaults'}
              >
                <strong>Defaults</strong>
                <span class="text-xs text-muted">Use models from Settings</span>
              </button>
              <button
                class="strategy-btn"
                class:active={modelStrategy === 'custom'}
                on:click={() => modelStrategy = 'custom'}
              >
                <strong>Custom</strong>
                <span class="text-xs text-muted">Choose per role for this project</span>
              </button>
            </div>
          </div>

          {#if modelStrategy === 'custom'}
            <div class="custom-roles">
              {#each Object.entries(customRoles) as [role, cfg]}
                {@const info = roleLabels[role] || { label: role, hint: '' }}
                <div class="custom-role-row">
                  <div class="custom-role-label">
                    <span class="text-sm">{info.label}</span>
                    <span class="text-xs text-dim">{info.hint}</span>
                  </div>
                  <select
                    class="custom-role-select"
                    value="{cfg.provider}/{cfg.model}"
                    on:change={(e) => handleCustomRoleChange(role, e.currentTarget.value)}
                  >
                    {#each availableModels as m}
                      <option value="{m.provider}/{m.id}">{m.provider}/{m.id}</option>
                    {/each}
                  </select>
                </div>
              {/each}
            </div>
          {/if}

          <div class="form-actions">
            <button class="btn" on:click={() => showNewProject = false}>Cancel</button>
            <button class="btn btn-primary" on:click={handleCreate} disabled={!newProjectBrief.trim() || creating}>
              {creating ? 'Creating...' : 'Create & Start'}
            </button>
          </div>
        </div>
      </div>
    {/if}

    {#if $projects.length === 0 && !showNewProject}
      <div class="empty-state">
        <div class="empty-icon">📦</div>
        <h2>No projects yet</h2>
        <p class="text-muted">Create your first project and watch Warpspawn build it autonomously.</p>
        {#if !$canAct}
          <p class="text-sm text-dim mt-2">Complete the setup wizard first to enable project creation.</p>
        {/if}
      </div>
    {:else}
      <div class="project-grid">
        {#each $projects as project (project.ID)}
          <ProjectCard {project} on:view={(e) => openProject(e.detail)} />
        {/each}
      </div>
    {/if}
  {/if}

</div>

<style>
  .dashboard {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .dashboard-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .new-project-form {
    animation: slideDown 0.15s ease-out;
  }
  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-8px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .project-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 12px;
  }
  .empty-state {
    text-align: center;
    padding: 60px 20px;
  }
  .empty-icon {
    font-size: 3rem;
    margin-bottom: 12px;
    opacity: 0.6;
  }

  /* Strategy selector */
  .strategy-section {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .strategy-options {
    display: flex;
    gap: 8px;
  }
  .strategy-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 5px 12px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    cursor: pointer;
    font-size: 0.8rem;
    transition: all 0.15s;
  }
  .strategy-btn:hover {
    border-color: rgba(255, 255, 255, 0.12);
    color: var(--text);
  }
  .strategy-btn.active {
    border-color: rgba(114, 230, 184, 0.3);
    color: var(--text);
    background: var(--bg-elevated);
  }
  .strategy-btn strong {
    font-weight: 500;
  }

  .form-actions {
    display: flex;
    gap: 8px;
    justify-content: space-between;
    margin-top: 8px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }

  /* Custom role picker */
  .custom-roles {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    background: var(--bg);
    border-radius: var(--radius-sm);
    animation: slideDown 0.12s ease-out;
  }
  .custom-role-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
  }
  .custom-role-label {
    display: flex;
    flex-direction: column;
    min-width: 160px;
  }
  .custom-role-select {
    flex: 1;
    max-width: 260px;
    font-family: var(--font-mono);
    font-size: 0.8rem;
  }
</style>
