<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { addNotification } from '../stores/app';
  import { afterUpdate } from 'svelte';

  export let projectId: string;
  export let initialMode: 'quick' | 'guided' = 'quick';
  export let modelName: string = '';
  export let existingMessages: ChatMsg[] | null = null;
  export let existingPhase: string = '';

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
      <span class="badge" class:badge-blue={phase === 'shaping'} class:badge-amber={phase === 'plan-review'} class:badge-green={phase === 'approved'}>
        {phase === 'shaping' ? 'Shaping' : phase === 'plan-review' ? 'Plan Review' : 'Approved'}
      </span>
    </div>
    {#if buildRunning}
      <span class="badge badge-blue"><span class="pulse-dot-inline"></span> Building...</span>
    {:else if phase === 'approved' || phase === 'plan-review'}
      <button class="btn btn-primary btn-sm" on:click={startBuild} disabled={loading}>
        {loading ? 'Starting...' : 'Start Building →'}
      </button>
    {/if}
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
