<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { addNotification } from '../stores/app';
  import { afterUpdate } from 'svelte';

  export let projectId: string;
  export let initialMode: 'quick' | 'guided' = 'quick';

  const dispatch = createEventDispatcher();

  interface ChatMsg {
    role: string;
    content: string;
    timestamp: number;
  }

  let messages: ChatMsg[] = [];
  let input = '';
  let loading = false;
  let phase = 'shaping';
  let started = false;
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
      if (data.reply) {
        messages = [...messages, { role: 'assistant', content: data.reply, timestamp: Date.now() }];
      }
      phase = data.phase || 'shaping';
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

      if (data.reply) {
        messages = [...messages, { role: 'assistant', content: data.reply, timestamp: Date.now() }];
      }

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
    try {
      const resp = await fetch(`/api/project/${projectId}/build`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}`,
          'Content-Type': 'application/json',
        },
      });
      if (!resp.ok) throw new Error(await resp.text());
      addNotification('success', 'Build started');
      dispatch('build-started');
    } catch (e: any) {
      addNotification('error', `Failed to start build: ${e.message}`);
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
    {#if phase === 'approved' || phase === 'plan-review'}
      <button class="btn btn-primary btn-sm" on:click={startBuild}>
        Start Building →
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
              <pre class="msg-text">{msg.content}</pre>
            </div>
          </div>
        {:else if msg.role === 'user'}
          <div class="msg msg-user">
            <div class="msg-content">
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
        disabled={loading || phase === 'approved'}
      ></textarea>
      <button class="btn btn-primary" on:click={sendMessage} disabled={!input.trim() || loading || phase === 'approved'}>
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
