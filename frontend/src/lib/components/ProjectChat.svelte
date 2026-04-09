<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { addNotification, agentLog } from '../stores/app';
  import { afterUpdate } from 'svelte';

  export let projectId: string;
  export let initialMode: 'quick' | 'guided' = 'quick';
  export let modelName: string = '';
  export let existingMessages: ChatMsg[] | null = null;
  export let existingPhase: string = '';
  export let totalTasks: number = 0;
  export let doneTasks: number = 0;

  const dispatch = createEventDispatcher();

  interface ChatMsg {
    role: string;
    content: string;
    timestamp: number;
  }

  function formatTime(ts: number): string {
    if (!ts) return '';
    const d = new Date(ts);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  let messages: ChatMsg[] = existingMessages || [];
  let input = '';
  let loading = false;
  let phase = existingPhase || 'shaping';
  let started = existingMessages != null && existingMessages.length > 0;
  let buildRunning = false;
  let buildCompleted = false;
  let buildTriggered = false;
  let needsInput = false;

  $: hasUnfinishedTasks = totalTasks > 0 && doneTasks < totalTasks;
  $: hasSomeDone = doneTasks > 0;
  $: hasTasks = totalTasks > 0;
  $: planReady = phase === 'approved' || phase === 'plan-review';
  // Continue: has tasks, some done, not all done
  $: showContinueButton = hasUnfinishedTasks && !buildRunning && hasSomeDone;
  // Start: has tasks but none started
  $: showStartButton = hasTasks && !buildRunning && !hasSomeDone && planReady;
  // First start: plan just approved, no tasks yet
  $: showFirstStart = planReady && !buildRunning && !hasTasks && !buildTriggered;

  // Detect build completion and errors from agent log
  const unsubLog = agentLog.subscribe((log) => {
    if (buildRunning) {
      if (log.some(e => e.type === 'complete' && e.content.includes('Build finished'))) {
        buildRunning = false;
        buildCompleted = true;
        needsInput = false;
      }
      if (log.some(e => e.type === 'error' || (e.type === 'text' && (e.content.includes('failed') || e.content.includes('cancelled') || e.content.includes('budget'))))) {
        buildRunning = false;
        needsInput = true;
      }
    }
  });
  onDestroy(() => unsubLog());
  let chatContainer: HTMLDivElement;

  afterUpdate(() => {
    if (chatContainer) {
      chatContainer.scrollTop = chatContainer.scrollHeight;
    }
  });

  async function startChat() {
    started = true;
    loading = true;
    try {
      const resp = await fetch(`/api/project/${projectId}/chat`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ mode: initialMode, message: '' }),
      });
      if (!resp.ok) throw new Error(await resp.text());
      const data = await resp.json();
      messages = data.messages || [];
      phase = data.phase || 'shaping';
      if (data.model) modelName = data.model;
    } catch (e: any) {
      addNotification('error', `Chat error: ${e.message}`);
    } finally {
      loading = false;
    }
  }

  async function sendMessage() {
    if (!input.trim() || loading) return;
    const msg = input.trim();
    input = '';

    messages = [...messages, { role: 'user', content: msg, timestamp: Date.now() }];
    loading = true;

    try {
      const resp = await fetch(`/api/project/${projectId}/chat`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ message: msg }),
      });
      if (!resp.ok) throw new Error(await resp.text());
      const data = await resp.json();
      phase = data.phase || phase;
      messages = data.messages || messages;
      if (data.model) modelName = data.model;

      if (phase === 'approved') {
        addNotification('success', 'Plan approved — tasks created. Ready to build.');
        dispatch('approved');
      }
    } catch (e: any) {
      addNotification('error', `Chat error: ${e.message}`);
    } finally {
      loading = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }

  async function pauseBuild() {
    try {
      const resp = await fetch(`/api/run/abort`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({ project_id: projectId }),
      });
      if (resp.ok) {
        buildRunning = false;
        needsInput = true;
        addNotification('warning', 'Build paused');
      }
    } catch { /* ignore */ }
  }

  function changePlan() {
    phase = 'plan-review';
    buildTriggered = false;
    buildCompleted = false;
    needsInput = false;
    addNotification('info', 'You can now modify the plan. Type your changes and MC will update it.');
  }

  async function startBuild() {
    loading = true;
    try {
      // Step 1: Approve the plan (create tasks) if not already approved
      if (phase === 'plan-review') {
        const approveResp = await fetch(`/api/project/${projectId}/chat`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ message: 'Approved.' }),
        });
        if (!approveResp.ok) throw new Error(await approveResp.text());
        const approveData = await approveResp.json();
        messages = approveData.messages || messages;
        phase = approveData.phase || phase;
      }

      // Step 2: Start the build
      const buildResp = await fetch(`/api/project/${projectId}/build`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}`,
          'Content-Type': 'application/json',
        },
      });
      if (!buildResp.ok) throw new Error(await buildResp.text());
      buildRunning = true;
      buildCompleted = false;
      buildTriggered = true;
      addNotification('success', 'Build started — agents are working autonomously');
      dispatch('build-started');
    } catch (e: any) {
      addNotification('error', `Failed to start build: ${e.message}`);
    } finally {
      loading = false;
    }
  }
</script>

<div class="chat-panel card">
  <div class="chat-header">
    <div class="flex items-center gap-2">
      <span>🎛️</span>
      <strong class="text-sm">Mission Control</strong>
      <span class="badge"
        class:badge-blue={buildRunning || phase === 'shaping'}
        class:badge-amber={phase === 'plan-review' && !buildRunning && !buildCompleted && !needsInput}
        class:badge-green={(buildCompleted || (phase === 'approved' && !needsInput)) && !buildRunning}
        class:badge-red={needsInput}>
        {#if needsInput}
          User input required
        {:else if buildRunning}
          Building
        {:else if buildCompleted}
          Complete
        {:else if phase === 'shaping'}
          Shaping
        {:else if phase === 'plan-review'}
          Plan Review
        {:else if phase === 'approved'}
          Ready
        {:else}
          Chat
        {/if}
      </span>
    </div>
    <div class="flex gap-2">
      {#if buildRunning}
        <button class="btn btn-danger btn-sm" on:click={pauseBuild}>Pause Building</button>
      {:else if !hasUnfinishedTasks && totalTasks > 0 && doneTasks === totalTasks}
        <span class="badge badge-green">Build complete</span>
      {:else if showContinueButton || needsInput}
        <button class="btn btn-primary btn-sm" on:click={startBuild} disabled={loading}>
          {loading ? 'Starting...' : `Continue Building (${totalTasks - doneTasks} remaining) →`}
        </button>
        <button class="btn btn-sm" on:click={changePlan}>Change Plan</button>
      {:else if showStartButton}
        <button class="btn btn-primary btn-sm" on:click={startBuild} disabled={loading}>
          {loading ? 'Starting...' : 'Start Building →'}
        </button>
        <button class="btn btn-sm" on:click={changePlan}>Change Plan</button>
      {:else if showFirstStart}
        <button class="btn btn-primary btn-sm" on:click={startBuild} disabled={loading}>
          {loading ? 'Starting...' : 'Start Building →'}
        </button>
      {/if}
    </div>
  </div>

  <div class="chat-messages" bind:this={chatContainer}>
    {#if !started}
      <div class="chat-start">
        <p class="text-muted text-sm">Mission Control will {initialMode === 'quick' ? 'create a plan from your brief' : 'ask questions to refine scope, then create a plan'}.</p>
        <button class="btn btn-primary" on:click={startChat}>
          {initialMode === 'quick' ? 'Generate Plan' : 'Start Shaping'}
        </button>
      </div>
    {:else}
      {#each messages as msg}
        {#if msg.role === 'assistant'}
          <div class="msg msg-assistant">
            <span class="msg-avatar">🎛️</span>
            <div class="msg-content">
              <div class="msg-meta">
                <span class="msg-sender">Mission Control</span>
                {#if modelName}
                  <span class="msg-model">{modelName}</span>
                {/if}
                {#if msg.timestamp}
                  <span class="msg-time">{formatTime(msg.timestamp)}</span>
                {/if}
              </div>
              <pre class="msg-text">{msg.content}</pre>
            </div>
          </div>
        {:else if msg.role === 'user'}
          <div class="msg msg-user">
            <div class="msg-content">
              <div class="msg-meta msg-meta-right">
                {#if msg.timestamp}
                  <span class="msg-time">{formatTime(msg.timestamp)}</span>
                {/if}
                <span class="msg-sender">You</span>
              </div>
              <pre class="msg-text">{msg.content}</pre>
            </div>
            <span class="msg-avatar">👤</span>
          </div>
        {/if}
      {/each}
      {#if loading}
        <div class="msg msg-assistant">
          <span class="msg-avatar">🎛️</span>
          <div class="msg-content">
            <span class="typing">Thinking...</span>
          </div>
        </div>
      {/if}
    {/if}
  </div>

  {#if started}
    <div class="chat-input">
      <textarea
        bind:value={input}
        on:keydown={handleKeydown}
        placeholder={phase === 'plan-review' ? 'Approve, request changes, or ask questions...' : 'Type a message...'}
        rows="2"
        disabled={loading}
      ></textarea>
      <button class="btn btn-primary" on:click={sendMessage} disabled={!input.trim() || loading}>
        Send
      </button>
    </div>
  {/if}
</div>

<style>
  .chat-panel {
    display: flex;
    flex-direction: column;
    padding: 0;
    overflow: hidden;
    max-height: 500px;
  }
  .chat-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-elevated);
    flex-shrink: 0;
  }
  .chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-height: 150px;
  }
  .chat-start {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 24px;
    text-align: center;
  }
  .msg {
    display: flex;
    gap: 8px;
    max-width: 85%;
  }
  .msg-user {
    align-self: flex-end;
  }
  .msg-assistant {
    align-self: flex-start;
  }
  .msg-avatar {
    flex-shrink: 0;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.9rem;
  }
  .msg-content {
    background: var(--bg-elevated);
    border-radius: var(--radius-sm);
    padding: 8px 12px;
  }
  .msg-user .msg-content {
    background: var(--accent-dim);
  }
  .msg-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 4px;
  }
  .msg-meta-right {
    justify-content: flex-end;
  }
  .msg-sender {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-muted);
  }
  .msg-model {
    font-size: 0.65rem;
    font-family: var(--font-mono);
    color: var(--text-dim);
    background: var(--bg);
    padding: 1px 5px;
    border-radius: 3px;
  }
  .msg-time {
    font-size: 0.65rem;
    color: var(--text-dim);
  }
  .msg-text {
    font-family: var(--font-sans);
    font-size: 0.85rem;
    line-height: 1.5;
    white-space: pre-wrap;
    word-wrap: break-word;
    margin: 0;
    color: var(--text);
  }
  .typing {
    color: var(--text-dim);
    font-style: italic;
    font-size: 0.85rem;
    animation: pulse 1.5s infinite;
  }
  .pulse-dot-inline {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    animation: pulse 1.5s infinite;
    margin-right: 2px;
    vertical-align: middle;
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }
  .chat-input {
    display: flex;
    gap: 8px;
    padding: 10px 14px;
    border-top: 1px solid var(--border);
    background: var(--bg-surface);
    flex-shrink: 0;
  }
  .chat-input textarea {
    flex: 1;
    min-height: unset;
    resize: none;
  }
  .chat-input .btn {
    align-self: flex-end;
  }
</style>
