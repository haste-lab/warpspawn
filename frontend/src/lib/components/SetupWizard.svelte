<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { settings } from '../stores/app';
  import { testProvider, updateSettings } from '../api';
  import type { AppSettings } from '../api';

  const dispatch = createEventDispatcher();

  let step = 1;
  let ollamaUrl = 'http://localhost:11434';
  let ollamaStatus: 'idle' | 'testing' | 'ok' | 'fail' = 'idle';
  let ollamaModels: string[] = [];
  let ollamaError = '';

  let openaiKey = '';
  let openaiStatus: 'idle' | 'testing' | 'ok' | 'fail' = 'idle';
  let anthropicKey = '';
  let anthropicStatus: 'idle' | 'testing' | 'ok' | 'fail' = 'idle';

  let builderProvider = 'ollama';
  let builderModel = '';
  let reviewerProvider = 'ollama';
  let reviewerModel = '';
  let dailyLimit = 10;

  let saving = false;

  async function testOllama() {
    ollamaStatus = 'testing';
    ollamaError = '';
    try {
      const result = await testProvider('ollama', { base_url: ollamaUrl });
      if (result.ok) {
        ollamaStatus = 'ok';
        ollamaModels = result.models || [];
        if (ollamaModels.length > 0 && !builderModel) {
          builderModel = ollamaModels.find(m => m.includes('coder')) || ollamaModels[0];
          reviewerModel = ollamaModels.find(m => m.includes('qwen3') || m.includes('llama')) || ollamaModels[0];
        }
      } else {
        ollamaStatus = 'fail';
        ollamaError = result.error || 'Connection failed';
      }
    } catch (e: any) {
      ollamaStatus = 'fail';
      ollamaError = e.message;
    }
  }

  async function testOpenAI() {
    openaiStatus = 'testing';
    try {
      const result = await testProvider('openai', { api_key: openaiKey });
      openaiStatus = result.ok ? 'ok' : 'fail';
    } catch {
      openaiStatus = 'fail';
    }
  }

  async function testAnthropic() {
    anthropicStatus = 'testing';
    try {
      const result = await testProvider('anthropic', { api_key: anthropicKey });
      anthropicStatus = result.ok ? 'ok' : 'fail';
    } catch {
      anthropicStatus = 'fail';
    }
  }

  $: hasProvider = ollamaStatus === 'ok' || openaiStatus === 'ok' || anthropicStatus === 'ok';
  $: allModels = ollamaModels;

  async function finish() {
    saving = true;
    try {
      const updated: Record<string, unknown> = {
        config_version: 1,
        providers: {
          ollama: { enabled: ollamaStatus === 'ok', base_url: ollamaUrl },
          openai: { enabled: openaiStatus === 'ok', key_ref: 'keyring:warpspawn/openai' },
          anthropic: { enabled: anthropicStatus === 'ok', key_ref: 'keyring:warpspawn/anthropic' },
        },
        roles: {
          'mission-control': { provider: builderProvider, model: reviewerModel || builderModel || 'qwen3:8b' },
          'architect': { provider: builderProvider, model: reviewerModel || builderModel || 'qwen3:8b' },
          'ux': { provider: builderProvider, model: reviewerModel || builderModel || 'qwen3:8b' },
          'builder': { provider: builderProvider, model: builderModel || 'qwen2.5-coder:7b' },
          'builder-light': { provider: builderProvider, model: builderModel || 'qwen2.5-coder:7b' },
          'reviewer-qa': { provider: builderProvider, model: reviewerModel || builderModel || 'qwen3:8b' },
        },
        budget: { daily_limit_usd: dailyLimit },
        execution: { max_tool_calls: 30, agent_timeout_s: 240, shell_mode: 'restricted' },
      };

      const result = await updateSettings(updated as any);
      settings.set(result);
      dispatch('complete');
    } catch (e: any) {
      alert('Failed to save settings: ' + e.message);
    } finally {
      saving = false;
    }
  }
</script>

<div class="wizard">
  <div class="wizard-header">
    <div>
      <h1>Welcome to Warpspawn</h1>
      <p class="text-muted mt-2">Let's configure your LLM providers so agents can build your projects.</p>
    </div>
    <button class="skip-link" on:click={() => dispatch('close')}>
      Skip for now, explore the UI →
    </button>
  </div>

  <div class="steps">
    <div class="step" class:active={step === 1} class:done={step > 1}>
      <span class="step-num">{step > 1 ? '✓' : '1'}</span> Providers
    </div>
    <div class="step-line"></div>
    <div class="step" class:active={step === 2} class:done={step > 2}>
      <span class="step-num">{step > 2 ? '✓' : '2'}</span> Models
    </div>
    <div class="step-line"></div>
    <div class="step" class:active={step === 3}>
      <span class="step-num">3</span> Budget
    </div>
  </div>

  {#if step === 1}
    <div class="wizard-body">
      <h2>LLM Providers</h2>
      <p class="text-muted mb-2">Connect at least one provider. Ollama is free and runs locally.</p>

      <div class="provider-card">
        <div class="provider-header">
          <div>
            <strong>Ollama</strong>
            <span class="badge badge-green">Local · Free</span>
          </div>
          {#if ollamaStatus === 'ok'}
            <span class="badge badge-green">✓ Connected ({ollamaModels.length} models)</span>
          {:else if ollamaStatus === 'fail'}
            <span class="badge badge-red">✗ {ollamaError}</span>
          {/if}
        </div>
        <div class="provider-body">
          <div class="flex gap-2 items-center">
            <div style="flex:1">
              <label>URL</label>
              <input type="text" bind:value={ollamaUrl} placeholder="http://localhost:11434" />
            </div>
            <button class="btn" on:click={testOllama} disabled={ollamaStatus === 'testing'}>
              {ollamaStatus === 'testing' ? 'Testing...' : 'Test Connection'}
            </button>
          </div>
        </div>
      </div>

      <div class="provider-card">
        <div class="provider-header">
          <div>
            <strong>OpenAI</strong>
            <span class="badge badge-blue">Cloud · Paid</span>
          </div>
          {#if openaiStatus === 'ok'}
            <span class="badge badge-green">✓ Valid</span>
          {:else if openaiStatus === 'fail'}
            <span class="badge badge-red">✗ Invalid key</span>
          {/if}
        </div>
        <div class="provider-body">
          <div class="flex gap-2 items-center">
            <div style="flex:1">
              <label>API Key</label>
              <input type="password" bind:value={openaiKey} placeholder="sk-..." />
            </div>
            <button class="btn" on:click={testOpenAI} disabled={!openaiKey || openaiStatus === 'testing'}>
              {openaiStatus === 'testing' ? 'Testing...' : 'Test'}
            </button>
          </div>
        </div>
      </div>

      <div class="provider-card">
        <div class="provider-header">
          <div>
            <strong>Anthropic</strong>
            <span class="badge badge-blue">Cloud · Paid</span>
          </div>
          {#if anthropicStatus === 'ok'}
            <span class="badge badge-green">✓ Valid</span>
          {:else if anthropicStatus === 'fail'}
            <span class="badge badge-red">✗ Invalid key</span>
          {/if}
        </div>
        <div class="provider-body">
          <div class="flex gap-2 items-center">
            <div style="flex:1">
              <label>API Key</label>
              <input type="password" bind:value={anthropicKey} placeholder="sk-ant-..." />
            </div>
            <button class="btn" on:click={testAnthropic} disabled={!anthropicKey || anthropicStatus === 'testing'}>
              {anthropicStatus === 'testing' ? 'Testing...' : 'Test'}
            </button>
          </div>
        </div>
      </div>

      <div class="wizard-footer">
        <div></div>
        <button class="btn btn-primary" on:click={() => step = 2} disabled={!hasProvider}>
          Next: Models →
        </button>
      </div>
    </div>

  {:else if step === 2}
    <div class="wizard-body">
      <h2>Model Assignment</h2>
      <p class="text-muted mb-2">Choose which model each role uses. Lighter models = lower cost.</p>

      <div class="model-grid">
        <div class="model-row">
          <label>Builder (writes code)</label>
          <select bind:value={builderModel}>
            {#each allModels as m}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </div>
        <div class="model-row">
          <label>Reviewer / QA (validates work)</label>
          <select bind:value={reviewerModel}>
            {#each allModels as m}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </div>
      </div>

      <div class="hint">
        <strong>Tip:</strong> Use a coder model (e.g. qwen2.5-coder) for Builder and a general model (e.g. qwen3) for Reviewer. The runtime auto-selects between light and standard tiers based on task complexity.
      </div>

      <div class="wizard-footer">
        <button class="btn" on:click={() => step = 1}>← Back</button>
        <button class="btn btn-primary" on:click={() => step = 3}>Next: Budget →</button>
      </div>
    </div>

  {:else if step === 3}
    <div class="wizard-body">
      <h2>Budget</h2>
      <p class="text-muted mb-2">Set a daily spending limit. Local models (Ollama) are free. Cloud models cost per token.</p>

      <div class="budget-input">
        <label>Daily limit (USD)</label>
        <div class="flex gap-2 items-center">
          <input type="number" bind:value={dailyLimit} min="0.5" step="0.5" style="max-width: 120px" />
          <span class="text-muted text-sm">$ / day</span>
        </div>
      </div>

      <div class="budget-presets">
        <button class="btn btn-sm" on:click={() => dailyLimit = 1}>$1 (minimal)</button>
        <button class="btn btn-sm" on:click={() => dailyLimit = 5}>$5 (light)</button>
        <button class="btn btn-sm" on:click={() => dailyLimit = 10}>$10 (standard)</button>
        <button class="btn btn-sm" on:click={() => dailyLimit = 25}>$25 (heavy)</button>
      </div>

      <div class="hint mt-4">
        Ollama models have zero cost. This limit only applies to cloud API calls.
      </div>

      <div class="wizard-footer">
        <button class="btn" on:click={() => step = 2}>← Back</button>
        <button class="btn btn-primary" on:click={finish} disabled={saving}>
          {saving ? 'Saving...' : 'Finish Setup ✓'}
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .wizard {
    max-width: 640px;
    margin: 0 auto;
  }
  .wizard-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 24px;
  }
  .skip-link {
    background: none;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.8rem;
    white-space: nowrap;
  }
  .skip-link:hover { color: var(--text-muted); }

  .steps {
    display: flex;
    align-items: center;
    gap: 0;
    margin-bottom: 24px;
  }
  .step {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.85rem;
    color: var(--text-dim);
    padding: 6px 12px;
    border-radius: var(--radius-sm);
  }
  .step.active { color: var(--text); background: var(--bg-elevated); }
  .step.done { color: var(--accent); }
  .step-num {
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background: var(--bg);
    font-size: 0.75rem;
    font-weight: 600;
  }
  .step.active .step-num { background: var(--accent); color: var(--bg); }
  .step.done .step-num { background: var(--accent-dim); }
  .step-line {
    flex: 1;
    height: 1px;
    background: var(--border);
  }

  .wizard-body { display: flex; flex-direction: column; gap: 16px; }
  .wizard-footer {
    display: flex;
    justify-content: space-between;
    margin-top: 8px;
  }

  .provider-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .provider-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 14px;
    background: var(--bg-elevated);
    font-size: 0.9rem;
  }
  .provider-header div { display: flex; align-items: center; gap: 8px; }
  .provider-body { padding: 12px 14px; }

  .model-grid { display: flex; flex-direction: column; gap: 12px; }
  .model-row { display: flex; flex-direction: column; gap: 4px; }
  .model-row label { font-weight: 500; color: var(--text); font-size: 0.85rem; }

  .hint {
    padding: 10px 14px;
    background: var(--blue-dim);
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
    color: var(--blue);
  }

  .budget-presets {
    display: flex;
    gap: 8px;
    margin-top: 4px;
  }
</style>
