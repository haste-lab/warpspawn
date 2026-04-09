<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { getProjectDetail } from '../api';
  import ProjectChat from './ProjectChat.svelte';

  export let projectId: string;

  const dispatch = createEventDispatcher();

  let detail: ProjectDetailData | null = null;
  let loading = true;
  let shapingMode: 'quick' | 'guided' = 'quick';
  let showModeChoice = true;
  let error = '';

  interface TaskInfo {
    id: string;
    title: string;
    status: string;
    priority: string;
    owner_role: string;
  }

  interface ProjectDetailData {
    id: string;
    name: string;
    path: string;
    lifecycle: string;
    current_stage: string;
    objective: string;
    brief: string;
    total_tasks: number;
    done_tasks: number;
    tasks: TaskInfo[];
    stats: {
      total_runs: number;
      total_input_tokens: number;
      total_output_tokens: number;
      total_cost_usd: number;
      total_tool_calls: number;
    };
  }

  onMount(async () => {
    try {
      detail = await getProjectDetail(projectId);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  $: progressPct = detail && detail.total_tasks > 0 ? (detail.done_tasks / detail.total_tasks) * 100 : 0;

  function statusBadge(status: string): string {
    const map: Record<string, string> = {
      'done': 'badge-green', 'archived': 'badge-green',
      'in-build': 'badge-blue', 'ready-for-build': 'badge-blue',
      'in-review': 'badge-amber', 'rework': 'badge-amber',
      'blocked': 'badge-red',
      'intake': 'badge-dim', 'shaping': 'badge-dim',
    };
    return map[status] || 'badge-dim';
  }

  function formatTokens(n: number): string {
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
    if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
    return String(n);
  }
</script>

<div class="detail">
  <button class="back-btn" on:click={() => dispatch('back')}>
    ← Back to projects
  </button>

  {#if loading}
    <p class="text-muted">Loading project...</p>
  {:else if error}
    <p class="text-muted">Error: {error}</p>
  {:else if detail}
    <div class="detail-header">
      <div>
        <h1>{detail.name || detail.id}</h1>
        <span class="badge {statusBadge(detail.current_stage || detail.lifecycle)}">
          {detail.current_stage || detail.lifecycle}
        </span>
      </div>
      <button class="btn btn-primary">Run Next Task</button>
    </div>

    {#if detail.objective}
      <div class="card">
        <h3 class="mb-2">Objective</h3>
        <p class="objective-text">{detail.objective}</p>
      </div>
    {/if}

    <!-- Shaping chat (shown for intake/shaping projects) -->
    {#if detail.current_stage === 'intake' || detail.current_stage === 'shaping' || (detail.total_tasks === 0 && detail.current_stage !== 'done')}
      {#if showModeChoice}
        <div class="card">
          <h3 class="mb-2">Plan your project</h3>
          <p class="text-muted text-sm mb-2">Mission Control will create a task plan from your brief. Choose how:</p>
          <div class="mode-options">
            <button class="mode-btn" class:active={shapingMode === 'quick'} on:click={() => shapingMode = 'quick'}>
              <strong>Quick Start</strong>
              <span class="text-xs text-muted">Generate a plan immediately. You review and approve before building starts.</span>
            </button>
            <button class="mode-btn" class:active={shapingMode === 'guided'} on:click={() => shapingMode = 'guided'}>
              <strong>Guided Shaping</strong>
              <span class="text-xs text-muted">MC asks questions first, proposes alternatives, then creates a plan.</span>
            </button>
          </div>
          <button class="btn btn-primary mt-2" on:click={() => showModeChoice = false}>
            Continue with {shapingMode === 'quick' ? 'Quick Start' : 'Guided Shaping'}
          </button>
        </div>
      {:else}
        <ProjectChat
          {projectId}
          initialMode={shapingMode}
          on:approved={async () => { detail = await getProjectDetail(projectId); }}
          on:build-started={async () => { detail = await getProjectDetail(projectId); }}
        />
      {/if}
    {/if}

    <!-- Stats row -->
    <div class="stats-row">
      <div class="stat-card card">
        <span class="stat-value">{detail.total_tasks}</span>
        <span class="stat-label">Tasks</span>
      </div>
      <div class="stat-card card">
        <span class="stat-value">{detail.done_tasks}</span>
        <span class="stat-label">Completed</span>
      </div>
      <div class="stat-card card">
        <span class="stat-value">{detail.stats.total_runs}</span>
        <span class="stat-label">Agent Runs</span>
      </div>
      <div class="stat-card card">
        <span class="stat-value">{formatTokens(detail.stats.total_input_tokens + detail.stats.total_output_tokens)}</span>
        <span class="stat-label">Tokens Used</span>
      </div>
      <div class="stat-card card">
        <span class="stat-value">${detail.stats.total_cost_usd.toFixed(2)}</span>
        <span class="stat-label">Cost</span>
      </div>
      <div class="stat-card card">
        <span class="stat-value">{detail.stats.total_tool_calls}</span>
        <span class="stat-label">Tool Calls</span>
      </div>
    </div>

    <!-- Progress -->
    <div class="card">
      <div class="flex justify-between items-center mb-2">
        <h3>Progress</h3>
        <span class="text-sm text-muted">{detail.done_tasks}/{detail.total_tasks} tasks done</span>
      </div>
      <div class="progress-bar-lg">
        <div class="progress-fill-lg" style="width: {progressPct}%"></div>
      </div>
    </div>

    <!-- Task list -->
    <div class="card">
      <h3 class="mb-2">Tasks</h3>
      {#if detail.tasks && detail.tasks.length > 0}
        <div class="task-list">
          {#each detail.tasks as task}
            <div class="task-row">
              <div class="flex items-center gap-2">
                {#if task.status === 'done' || task.status === 'archived'}
                  <span class="task-icon done">✓</span>
                {:else if task.status === 'in-build' || task.status === 'in-review'}
                  <span class="task-icon active">▸</span>
                {:else if task.status === 'blocked' || task.status === 'rework'}
                  <span class="task-icon warn">!</span>
                {:else}
                  <span class="task-icon pending">○</span>
                {/if}
                <span class="task-title">{task.title || task.id}</span>
              </div>
              <div class="flex items-center gap-2">
                <span class="badge {statusBadge(task.status)}">{task.status}</span>
                <span class="text-xs text-dim">{task.priority}</span>
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <p class="text-muted text-sm">No tasks yet.</p>
      {/if}
    </div>

    <!-- Brief (collapsible) -->
    <details class="card brief-section">
      <summary><h3>Project Brief</h3></summary>
      <pre class="brief-content">{detail.brief || 'No brief available.'}</pre>
    </details>
  {/if}
</div>

<style>
  .detail {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-width: 800px;
  }
  .back-btn {
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    font-size: 0.85rem;
    padding: 0;
    align-self: flex-start;
  }
  .back-btn:hover { color: var(--text); }
  .mode-options {
    display: flex;
    gap: 8px;
    margin-bottom: 4px;
  }
  .mode-btn {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 14px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text);
    cursor: pointer;
    text-align: left;
    transition: all 0.15s;
  }
  .mode-btn:hover {
    border-color: rgba(255, 255, 255, 0.12);
  }
  .mode-btn.active {
    border-color: rgba(114, 230, 184, 0.3);
    background: var(--bg-elevated);
  }
  .detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .detail-header > div {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .objective-text {
    color: var(--text);
    font-size: 0.95rem;
    line-height: 1.6;
  }
  .stats-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
    gap: 10px;
  }
  .stat-card {
    text-align: center;
    padding: 12px;
  }
  .stat-value {
    display: block;
    font-size: 1.4rem;
    font-weight: 700;
    font-family: var(--font-mono);
    color: var(--text);
  }
  .stat-label {
    font-size: 0.75rem;
    color: var(--text-dim);
  }
  .progress-bar-lg {
    height: 8px;
    background: var(--bg);
    border-radius: 999px;
    overflow: hidden;
  }
  .progress-fill-lg {
    height: 100%;
    background: var(--accent);
    border-radius: 999px;
    transition: width 0.3s;
  }
  .task-list {
    display: flex;
    flex-direction: column;
  }
  .task-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }
  .task-row:last-child { border-bottom: none; }
  .task-icon {
    width: 18px;
    text-align: center;
    font-size: 0.8rem;
    font-weight: 700;
  }
  .task-icon.done { color: var(--green); }
  .task-icon.active { color: var(--blue); }
  .task-icon.warn { color: var(--amber); }
  .task-icon.pending { color: var(--text-dim); }
  .task-title { font-size: 0.9rem; }
  .brief-section summary {
    cursor: pointer;
    list-style: none;
  }
  .brief-section summary::-webkit-details-marker { display: none; }
  .brief-section summary h3::before {
    content: '▸ ';
    color: var(--text-dim);
  }
  .brief-section[open] summary h3::before {
    content: '▾ ';
  }
  .brief-content {
    margin-top: 12px;
    font-family: var(--font-mono);
    font-size: 0.8rem;
    color: var(--text-muted);
    white-space: pre-wrap;
    line-height: 1.6;
    max-height: 400px;
    overflow-y: auto;
  }
</style>
