<script lang="ts">
  import { agentLog, activeRun } from '../stores/app';
  import { afterUpdate } from 'svelte';

  let logContainer: HTMLDivElement;
  let autoScroll = true;

  afterUpdate(() => {
    if (autoScroll && logContainer) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  });

  function handleScroll() {
    if (!logContainer) return;
    const atBottom = logContainer.scrollHeight - logContainer.scrollTop - logContainer.clientHeight < 40;
    autoScroll = atBottom;
  }
</script>

<div class="card agent-panel">
  <div class="agent-header">
    <div class="flex items-center gap-2">
      {#if $activeRun}
        <div class="pulse-dot"></div>
        <strong class="text-sm">Agent running</strong>
        <span class="text-xs text-muted">({$activeRun.role} on {$activeRun.projectId})</span>
      {:else}
        <strong class="text-sm">Agent Log</strong>
        <span class="text-xs text-muted">({$agentLog.length} entries)</span>
      {/if}
    </div>
    <div class="flex gap-2">
      {#if $activeRun}
        <button class="btn btn-danger btn-sm">Abort</button>
      {/if}
      <button class="btn btn-sm" on:click={() => agentLog.set([])}>Clear</button>
    </div>
  </div>

  <div class="log-output" bind:this={logContainer} on:scroll={handleScroll}>
    {#each $agentLog as entry (entry.id)}
      <div class="log-entry log-{entry.type}">
        {#if entry.type === 'tool_call'}
          <span class="log-prefix tool">▸ tool</span>
        {:else if entry.type === 'tool_result'}
          <span class="log-prefix result">◂ result</span>
        {:else if entry.type === 'error'}
          <span class="log-prefix error">✗ error</span>
        {:else if entry.type === 'complete'}
          <span class="log-prefix complete">✓ done</span>
        {/if}
        <span class="log-text">{entry.content}</span>
      </div>
    {/each}
    {#if $agentLog.length === 0}
      <div class="log-empty">No agent activity yet.</div>
    {/if}
  </div>
</div>

<style>
  .agent-panel {
    display: flex;
    flex-direction: column;
    gap: 0;
    padding: 0;
    overflow: hidden;
  }
  .agent-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-elevated);
  }
  .log-output {
    font-family: var(--font-mono);
    font-size: 0.8rem;
    line-height: 1.6;
    padding: 10px 14px;
    max-height: 300px;
    overflow-y: auto;
    background: var(--bg);
  }
  .log-entry {
    display: flex;
    gap: 8px;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .log-text { color: var(--text-muted); }
  .log-entry.log-text .log-text { color: var(--text); }
  .log-prefix {
    flex-shrink: 0;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .log-prefix.tool { color: var(--blue); }
  .log-prefix.result { color: var(--text-dim); }
  .log-prefix.error { color: var(--red); }
  .log-prefix.complete { color: var(--green); }
  .log-empty {
    color: var(--text-dim);
    font-style: italic;
  }
  .pulse-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--accent);
    animation: pulse 1.5s infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }
</style>
