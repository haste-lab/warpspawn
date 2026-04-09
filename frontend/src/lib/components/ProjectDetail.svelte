<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { getProjectDetail } from '../api';
  import { addNotification, projects, agentLog } from '../stores/app';
  import { getProjects, connectEvents } from '../api';
  import { handleSSEEvent } from '../stores/app';
  import ProjectChat from './ProjectChat.svelte';

  export let projectId: string;

  const dispatch = createEventDispatcher();

  let detail: ProjectDetailData | null = null;
  let loading = true;
  let error = '';
  let existingChat: { mode: string; messages: any[]; phase: string } | null = null;
  let showDeleteConfirm = false;
  let deleting = false;

  // For new projects without a chat yet
  let shapingMode: 'quick' | 'guided' = 'quick';
  let showModeChoice = false; // only shown for brand new intake projects

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

      // Check for existing chat
      try {
        const resp = await fetch(`/api/project/${projectId}/chat`, {
          headers: { 'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}` },
          credentials: 'same-origin',
        });
        if (resp.ok) {
          const chatData = await resp.json();
          if (chatData.messages && chatData.messages.length > 0) {
            existingChat = chatData;
            shapingMode = chatData.mode || 'quick';
          }
        }
      } catch { /* no existing chat */ }

      // Show mode choice only for brand-new intake projects with no chat history
      if ((detail.current_stage === 'intake' || detail.current_stage === 'shaping') &&
          detail.total_tasks === 0 && !existingChat) {
        showModeChoice = true;
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  $: progressPct = detail && detail.total_tasks > 0 ? (detail.done_tasks / detail.total_tasks) * 100 : 0;
  $: isIntake = detail && (detail.current_stage === 'intake' || detail.current_stage === 'shaping') && detail.total_tasks === 0;

  // Auto-refresh project detail when agent log updates (build in progress)
  let refreshTimer: ReturnType<typeof setTimeout> | null = null;
  let lastLogCount = 0;

  const unsubLog = agentLog.subscribe((log) => {
    if (log.length !== lastLogCount) {
      lastLogCount = log.length;
      // Debounce: refresh 1s after last log entry (avoid hammering during rapid updates)
      if (refreshTimer) clearTimeout(refreshTimer);
      refreshTimer = setTimeout(async () => {
        try {
          detail = await getProjectDetail(projectId);
        } catch { /* ignore refresh errors */ }
      }, 1500);
    }
  });

  onDestroy(() => {
    unsubLog();
    if (refreshTimer) clearTimeout(refreshTimer);
  });

  async function deleteProject() {
    deleting = true;
    try {
      const resp = await fetch(`/api/project/${projectId}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}` },
        credentials: 'same-origin',
      });
      if (!resp.ok) throw new Error(await resp.text());
      addNotification('success', `Project "${detail?.name || projectId}" deleted`);
      const updated = await getProjects();
      projects.set(updated);
      dispatch('back');
    } catch (e: any) {
      addNotification('error', `Delete failed: ${e.message}`);
    } finally {
      deleting = false;
      showDeleteConfirm = false;
    }
  }

  async function refreshDetail() {
    detail = await getProjectDetail(projectId);
  }

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

  function statusIcon(status: string): { icon: string; cls: string } {
    if (status === 'done' || status === 'archived') return { icon: '✓', cls: 'done' };
    if (status === 'in-build' || status === 'in-review') return { icon: '▸', cls: 'active' };
    if (status === 'blocked' || status === 'rework') return { icon: '!', cls: 'warn' };
    if (status === 'ready-for-build') return { icon: '○', cls: 'ready' };
    return { icon: '·', cls: 'pending' };
  }

  function formatTokens(n: number): string {
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
    if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
    return String(n);
  }
</script>

<div class="workspace">
  <button class="back-btn" on:click={() => dispatch('back')}>
    ← Back to projects
  </button>

  {#if loading}
    <p class="text-muted">Loading project...</p>
  {:else if error}
    <p class="text-muted">Error: {error}</p>
  {:else if detail}

    <!-- Header -->
    <div class="ws-header">
      <div>
        <h1>{detail.name || detail.id}</h1>
        <span class="badge {statusBadge(detail.current_stage || detail.lifecycle)}">
          {detail.current_stage || detail.lifecycle}
        </span>
      </div>
      <div class="flex gap-2">
        {#if !showDeleteConfirm}
          <button class="btn btn-danger btn-sm" on:click={() => showDeleteConfirm = true}>Delete</button>
        {/if}
      </div>
    </div>

    {#if showDeleteConfirm}
      <div class="card delete-confirm">
        <p><strong>Delete this project?</strong> This will permanently remove all project files, tasks, and history. This cannot be undone.</p>
        <div class="flex gap-2 mt-2">
          <button class="btn" on:click={() => showDeleteConfirm = false}>Cancel</button>
          <button class="btn btn-danger" on:click={deleteProject} disabled={deleting}>
            {deleting ? 'Deleting...' : 'Yes, delete permanently'}
          </button>
        </div>
      </div>
    {/if}

    <!-- Main content: two columns on wide screens -->
    <div class="ws-body">
      <!-- Left: project info -->
      <div class="ws-info">
        {#if detail.objective}
          <div class="card">
            <h3 class="mb-2">Objective</h3>
            <p class="objective-text">{detail.objective}</p>
          </div>
        {/if}

        <!-- Stats -->
        {#if detail.stats.total_runs > 0 || detail.total_tasks > 0}
          <div class="stats-row">
            <div class="stat-card card">
              <span class="stat-value">{detail.total_tasks}</span>
              <span class="stat-label">Tasks</span>
            </div>
            <div class="stat-card card">
              <span class="stat-value">{detail.done_tasks}</span>
              <span class="stat-label">Done</span>
            </div>
            <div class="stat-card card">
              <span class="stat-value">{detail.stats.total_runs}</span>
              <span class="stat-label">Runs</span>
            </div>
            <div class="stat-card card">
              <span class="stat-value">{formatTokens(detail.stats.total_input_tokens + detail.stats.total_output_tokens)}</span>
              <span class="stat-label">Tokens</span>
            </div>
          </div>
        {/if}

        <!-- Progress -->
        {#if detail.total_tasks > 0}
          <div class="card">
            <div class="flex justify-between items-center mb-2">
              <h3>Tasks</h3>
              <span class="text-xs text-muted">{detail.done_tasks}/{detail.total_tasks} done</span>
            </div>
            <div class="progress-bar-lg mb-2">
              <div class="progress-fill-lg" style="width: {progressPct}%"></div>
            </div>
            <div class="task-list">
              {#each detail.tasks as task}
                {@const si = statusIcon(task.status)}
                <div class="task-row">
                  <div class="task-left">
                    <span class="task-icon {si.cls}">{si.icon}</span>
                    <div class="task-info">
                      <span class="task-title">{task.title || task.id}</span>
                      <div class="task-pipeline">
                        {#each ['ready-for-build', 'in-build', 'in-review', 'done'] as stage}
                          {@const stageOrder = ['intake', 'shaping', 'ready-for-build', 'in-build', 'in-review', 'done']}
                          {@const currentIdx = stageOrder.indexOf(task.status === 'rework' ? 'in-build' : task.status)}
                          {@const stageIdx = stageOrder.indexOf(stage)}
                          <span class="pipe-stage"
                            class:pipe-done={stageIdx < currentIdx || task.status === 'done'}
                            class:pipe-active={stageIdx === currentIdx && task.status !== 'done'}
                            class:pipe-blocked={task.status === 'blocked' && stageIdx === currentIdx}
                          >{stage === 'ready-for-build' ? 'ready' : stage === 'in-build' ? 'build' : stage === 'in-review' ? 'review' : stage}</span>
                          {#if stage !== 'done'}<span class="pipe-arrow">→</span>{/if}
                        {/each}
                      </div>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Brief (collapsible) -->
        {#if detail.brief}
          <details class="card brief-section">
            <summary><h3>Project Brief</h3></summary>
            <pre class="brief-content">{detail.brief}</pre>
          </details>
        {/if}
      </div>
    </div>

    <!-- Mission Control chat — always at the bottom -->
    <div class="ws-chat">
      {#if showModeChoice}
        <div class="card">
          <h3 class="mb-2">Plan your project</h3>
          <p class="text-muted text-sm mb-2">Mission Control will create a task plan. Choose how:</p>
          <div class="mode-options">
            <button class="mode-btn" class:active={shapingMode === 'quick'} on:click={() => shapingMode = 'quick'}>
              <strong>Quick Start</strong>
              <span class="text-xs text-muted">Plan immediately, you approve</span>
            </button>
            <button class="mode-btn" class:active={shapingMode === 'guided'} on:click={() => shapingMode = 'guided'}>
              <strong>Guided</strong>
              <span class="text-xs text-muted">Q&A first, then plan</span>
            </button>
          </div>
          <button class="btn btn-primary mt-2" on:click={() => showModeChoice = false}>
            Continue
          </button>
        </div>
      {:else}
        <ProjectChat
          {projectId}
          initialMode={isIntake ? shapingMode : 'quick'}
          existingMessages={existingChat?.messages || null}
          existingPhase={existingChat?.phase || ''}
          on:approved={refreshDetail}
          on:build-started={refreshDetail}
        />
      {/if}
    </div>

  {/if}
</div>

<style>
  .workspace {
    display: flex;
    flex-direction: column;
    gap: 16px;
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

  .ws-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .ws-header > div {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .ws-body {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .ws-info {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .ws-chat {
    margin-top: 4px;
  }

  .delete-confirm {
    background: var(--red-dim);
    border-color: rgba(255, 123, 123, 0.2);
  }

  .objective-text {
    color: var(--text);
    font-size: 0.95rem;
    line-height: 1.6;
  }

  .stats-row {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
  }
  .stat-card {
    text-align: center;
    padding: 10px 8px;
  }
  .stat-value {
    display: block;
    font-size: 1.3rem;
    font-weight: 700;
    font-family: var(--font-mono);
    color: var(--text);
  }
  .stat-label {
    font-size: 0.7rem;
    color: var(--text-dim);
  }

  .progress-bar-lg {
    height: 6px;
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
    padding: 7px 0;
    border-bottom: 1px solid var(--border);
  }
  .task-row:last-child { border-bottom: none; }
  .task-left {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .task-icon {
    width: 18px;
    text-align: center;
    font-size: 0.8rem;
    font-weight: 700;
    flex-shrink: 0;
  }
  .task-icon.done { color: var(--green); }
  .task-icon.active { color: var(--blue); }
  .task-icon.warn { color: var(--amber); }
  .task-icon.ready { color: var(--blue); opacity: 0.6; }
  .task-icon.pending { color: var(--text-dim); }
  .task-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .task-title {
    font-size: 0.85rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .task-pipeline {
    display: flex;
    align-items: center;
    gap: 2px;
    margin-top: 2px;
  }
  .pipe-stage {
    font-size: 0.65rem;
    padding: 1px 5px;
    border-radius: 3px;
    color: var(--text-dim);
    background: var(--bg);
  }
  .pipe-stage.pipe-done {
    color: var(--green);
    background: var(--accent-dim);
  }
  .pipe-stage.pipe-active {
    color: var(--blue);
    background: var(--blue-dim);
    font-weight: 700;
  }
  .pipe-stage.pipe-blocked {
    color: var(--red);
    background: var(--red-dim);
    font-weight: 700;
  }
  .pipe-arrow {
    font-size: 0.6rem;
    color: var(--text-dim);
    opacity: 0.4;
  }

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
    max-height: 300px;
    overflow-y: auto;
  }

  .mode-options {
    display: flex;
    gap: 8px;
    margin-bottom: 4px;
  }
  .mode-btn {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 8px 12px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text);
    cursor: pointer;
    text-align: left;
    font-size: 0.85rem;
    transition: all 0.15s;
  }
  .mode-btn:hover { border-color: rgba(255, 255, 255, 0.12); }
  .mode-btn.active { border-color: rgba(114, 230, 184, 0.3); background: var(--bg-elevated); }
  .mb-2 { margin-bottom: 8px; }
</style>
