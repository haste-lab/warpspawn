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
    'needs attention': 'badge-red',
  }[project.CurrentStage?.toLowerCase()] || 'badge-dim';
</script>

<div class="card project-card">
  <div class="project-header">
    <h3 class="truncate">{project.Name || project.ID}</h3>
    <span class="badge {stageColor}">{project.CurrentStage || project.Lifecycle}</span>
  </div>

  <div class="project-tasks">
    <div class="progress-bar">
      <div class="progress-fill" style="width: {progressPct}%"></div>
    </div>
    <span class="text-xs text-muted">{project.DoneTasks}/{project.TotalTasks} tasks</span>
  </div>

  <div class="project-actions">
    <button class="btn btn-sm" on:click|stopPropagation={() => dispatch('view', project.ID)}>View</button>
    <button class="btn btn-sm btn-primary">Run Next</button>
  </div>
</div>

<style>
  .project-card {
    display: flex;
    flex-direction: column;
    gap: 12px;
    cursor: pointer;
    transition: border-color 0.15s;
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
  .project-actions {
    display: flex;
    gap: 6px;
  }
</style>
