<script lang="ts">
  import { projects, budget, activeRun, agentLog, canAct } from '../stores/app';
  import AgentPanel from './AgentPanel.svelte';
  import ProjectCard from './ProjectCard.svelte';

  let showNewProject = false;
  let newProjectBrief = '';
  let newProjectName = '';
  let creating = false;

  async function handleCreate() {
    if (!newProjectBrief.trim()) return;
    creating = true;
    // TODO: call createProject API
    creating = false;
    showNewProject = false;
    newProjectBrief = '';
    newProjectName = '';
  }
</script>

<div class="dashboard">
  <div class="dashboard-header">
    <h1>Projects</h1>
    <button class="btn btn-primary" on:click={() => showNewProject = !showNewProject} disabled={!$canAct}>
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
        <div class="flex gap-2 justify-between">
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
        <ProjectCard {project} />
      {/each}
    </div>
  {/if}

  {#if $agentLog.length > 0 || $activeRun}
    <div class="mt-4">
      <AgentPanel />
    </div>
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
</style>
