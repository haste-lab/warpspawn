<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type { ProjectSummary } from '../api';
  export let project: ProjectSummary;
  const dispatch = createEventDispatcher();

  $: progressPct = project.TotalTasks > 0 ? (project.DoneTasks / project.TotalTasks) * 100 : 0;
  $: stageColor = {
    'active': 'badge-blue',
    'done': 'badge-green',
    'blocked': 'badge-red',
    'ready for build': 'badge-blue',
    'in progress': 'badge-blue',
    'in review': 'badge-amber',
    'review ready': 'badge-amber',
    'rework': 'badge-amber',
    'intake': 'badge-dim',
    'shaping': 'badge-dim',
    'needs attention': 'badge-red',
  }[project.CurrentStage?.toLowerCase()] || 'badge-dim';
</script>

<button class="card project-card" on:click={() => dispatch('view', project.ID)}>
  <div class="project-header">
    <h3 class="truncate">{project.Name || project.ID}</h3>
    <span class="badge {stageColor}">{project.CurrentStage || project.Lifecycle}</span>
  </div>

  {#if project.TotalTasks > 0}
    <div class="project-tasks">
      <div class="progress-bar">
        <div class="progress-fill" style="width: {progressPct}%"></div>
      </div>
      <span class="text-xs text-muted">{project.DoneTasks}/{project.TotalTasks} tasks</span>
    </div>
  {:else}
    <p class="text-xs text-dim">No tasks yet — open to start planning</p>
  {/if}
</button>

<style>
  .project-card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    cursor: pointer;
    transition: border-color 0.15s;
    text-align: left;
    width: 100%;
    font: inherit;
    color: inherit;
  }
  .project-card:hover {
    border-color: var(--border-focus);
  }
  .project-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
  }
  .project-tasks {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .progress-bar {
    flex: 1;
    height: 4px;
    background: var(--bg);
    border-radius: 999px;
    overflow: hidden;
  }
  .progress-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 999px;
    transition: width 0.3s;
  }
</style>
