<script lang="ts">
  import { onMount } from 'svelte';
  import { settings, showWizard, addNotification } from '../stores/app';
  import { updateSettings } from '../api';

  interface ModelOption {
    provider: string;
    id: string;
    name: string;
  }

  let availableModels: ModelOption[] = [];
  let saving = false;

  onMount(async () => {
    try {
      const resp = await fetch('/api/models', {
        headers: { 'Authorization': `Bearer ${sessionStorage.getItem('ws_token') || ''}` }
      });
      if (resp.ok) {
        availableModels = await resp.json();
      }
    } catch { /* ignore */ }
  });

  function modelOptions(currentProvider: string): ModelOption[] {
    // Show all available models, grouped by provider
    return availableModels;
  }

  function modelDisplayName(provider: string, model: string): string {
    return `${provider}/${model}`;
  }

  async function handleRoleModelChange(role: string, value: string) {
    if (!$settings) return;
    // value format: "provider/model"
    const slashIdx = value.indexOf('/');
    if (slashIdx < 0) return;
    const provider = value.substring(0, slashIdx);
    const model = value.substring(slashIdx + 1);

    const updated = { ...$settings };
    updated.roles = { ...updated.roles };
    updated.roles[role] = { provider, model };

    saving = true;
    try {
      const result = await updateSettings(updated);
      settings.set(result);
      addNotification('success', `Updated ${role} → ${provider}/${model}`);
    } catch (e: any) {
      addNotification('error', `Failed to save: ${e.message}`);
    } finally {
      saving = false;
    }
  }

  interface RoleInfo {
    label: string;
    purpose: string;
    focus: string;
    modelHint: string;
    icon: string;
  }

  const roleDescriptions: Record<string, RoleInfo> = {
    'mission-control': {
      label: 'Mission Control',
      purpose: 'Orchestrates the entire delivery lifecycle',
      focus: 'Decomposes briefs into tasks, prioritizes work, routes between roles, closes completed work, escalates blockers',
      modelHint: 'Needs strong reasoning. Handles planning, not code generation.',
      icon: '🎛️',
    },
    'architect': {
      label: 'Architect',
      purpose: 'Defines technical structure and constraints',
      focus: 'System design, interfaces, data flow, non-functional requirements (performance, security, reliability)',
      modelHint: 'Text-focused reasoning. Does not generate code. A lighter model usually suffices.',
      icon: '🏗️',
    },
    'ux': {
      label: 'UX Designer',
      purpose: 'Defines user journeys and acceptance criteria',
      focus: 'User flows, interaction rules, empty/error/loading states, accessibility expectations, UI acceptance criteria',
      modelHint: 'Text-focused shaping. Does not generate code. A lighter model usually suffices.',
      icon: '✏️',
    },
    'builder': {
      label: 'Builder',
      purpose: 'Implements code for bounded tasks',
      focus: 'Writes code, creates files, runs commands, validates implementation. The only role that produces code artifacts.',
      modelHint: 'Needs strong code generation. Benefits most from a capable model. Highest token consumer.',
      icon: '🛠️',
    },
    'builder-light': {
      label: 'Builder (Light)',
      purpose: 'Implements simple, bounded tasks',
      focus: 'Same as Builder but auto-selected for small tasks (≤2 files, ≤4 criteria, existing code edits)',
      modelHint: 'A smaller/cheaper model for simple fixes, tweaks, and config changes.',
      icon: '🔧',
    },
    'reviewer-qa': {
      label: 'Reviewer / QA',
      purpose: 'Validates completed work against acceptance criteria',
      focus: 'Inspects changed files, verifies tests pass, checks acceptance criteria, writes review reports. Does NOT implement missing work.',
      modelHint: 'Needs code reading ability but does not generate code. A lighter model usually suffices.',
      icon: '✅',
    },
  };

  function getRoleInfo(role: string): RoleInfo {
    return roleDescriptions[role] || {
      label: role,
      purpose: 'Custom role',
      focus: '',
      modelHint: '',
      icon: '⚙️',
    };
  }
</script>

<div class="settings">
  <div class="settings-header">
    <h1>Settings</h1>
    <button class="btn btn-sm" on:click={() => showWizard.set(true)}>
      Run Setup Wizard
    </button>
  </div>

  {#if $settings}
    <section class="settings-section">
      <h2>Providers</h2>
      <div class="settings-grid">
        {#each Object.entries($settings.providers) as [name, config]}
          <div class="card">
            <div class="flex justify-between items-center">
              <div class="flex items-center gap-2">
                <strong>{name}</strong>
                {#if config.enabled}
                  <span class="badge badge-green">Enabled</span>
                {:else}
                  <span class="badge badge-dim">Disabled</span>
                {/if}
              </div>
              <label class="toggle">
                <input type="checkbox" checked={config.enabled} on:change={() => {/* TODO */}} />
                <span class="toggle-slider"></span>
              </label>
            </div>
            {#if config.base_url}
              <p class="text-xs text-muted mt-2 mono">{config.base_url}</p>
            {/if}
          </div>
        {/each}
      </div>
    </section>

    <section class="settings-section">
      <div class="section-intro">
        <h2>Role → Model Assignment</h2>
        <p class="text-muted text-sm">Each role has a specific purpose. Assign models based on what the role needs — code-generating roles need capable models, while planning/review roles can use lighter ones.</p>
      </div>

      <div class="role-grid">
        {#each Object.entries($settings.roles) as [role, config]}
          {@const info = getRoleInfo(role)}
          <div class="card role-card">
            <div class="role-header">
              <div class="role-identity">
                <span class="role-icon">{info.icon}</span>
                <div>
                  <strong>{info.label}</strong>
                  <p class="role-purpose">{info.purpose}</p>
                </div>
              </div>
            </div>

            <div class="role-detail">
              <div class="role-focus">
                <span class="detail-label">Focus</span>
                <span class="detail-text">{info.focus}</span>
              </div>
              <div class="role-hint">
                <span class="detail-label">Model guidance</span>
                <span class="detail-text">{info.modelHint}</span>
              </div>
            </div>

            <div class="role-model">
              <div class="model-select-row">
                <span class="detail-label">Model</span>
                {#if availableModels.length > 0}
                  <select
                    class="model-dropdown"
                    value="{config.provider}/{config.model}"
                    on:change={(e) => handleRoleModelChange(role, e.currentTarget.value)}
                    disabled={saving}
                  >
                    {#each availableModels as m}
                      <option value="{m.provider}/{m.id}">{m.provider}/{m.id}</option>
                    {/each}
                  </select>
                {:else}
                  <div class="flex gap-2 items-center">
                    <span class="badge badge-dim">{config.provider}</span>
                    <span class="mono text-sm">{config.model}</span>
                    <span class="text-xs text-dim">(no providers connected — models unavailable)</span>
                  </div>
                {/if}
              </div>
            </div>
          </div>
        {/each}
      </div>
    </section>

    <section class="settings-section">
      <h2>Execution</h2>
      <div class="card">
        <div class="settings-row">
          <div>
            <label>Max tool calls per agent</label>
            <p class="text-xs text-dim">Hard limit on tool invocations per run. Higher = more capable but more expensive.</p>
          </div>
          <input type="number" value={$settings.execution.max_tool_calls} style="max-width: 80px" />
        </div>
        <div class="settings-row">
          <div>
            <label>Agent timeout (seconds)</label>
            <p class="text-xs text-dim">Max wallclock time per agent run. Prevents runaway token consumption.</p>
          </div>
          <input type="number" value={$settings.execution.agent_timeout_s} style="max-width: 80px" />
        </div>
        <div class="settings-row">
          <div>
            <label>Shell execution mode</label>
            <p class="text-xs text-dim">Controls what shell commands agents can run on your machine.</p>
          </div>
          <select value={$settings.execution.shell_mode} style="max-width: 160px">
            <option value="unrestricted">Unrestricted</option>
            <option value="restricted">Restricted (allowlist)</option>
            <option value="approval">Approval required</option>
          </select>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <h2>Budget</h2>
      <div class="card">
        <div class="settings-row">
          <div>
            <label>Daily limit (USD)</label>
            <p class="text-xs text-dim">Ollama models are free. This limit only applies to cloud API calls (OpenAI, Anthropic).</p>
          </div>
          <div class="flex items-center gap-2">
            <span class="text-muted">$</span>
            <input type="number" value={$settings.budget.daily_limit_usd} min="0.5" step="0.5" style="max-width: 80px" />
          </div>
        </div>
      </div>
    </section>
  {:else}
    <p class="text-muted">Loading settings...</p>
  {/if}
</div>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    gap: 24px;
    max-width: 760px;
  }
  .settings-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .settings-section h2 {
    margin-bottom: 4px;
  }
  .section-intro {
    margin-bottom: 12px;
  }
  .section-intro p {
    margin-top: 4px;
  }
  .settings-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .role-grid {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .role-card {
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .role-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
  }
  .role-identity {
    display: flex;
    gap: 10px;
    align-items: flex-start;
  }
  .role-icon {
    font-size: 1.3rem;
    flex-shrink: 0;
    margin-top: 1px;
  }
  .role-purpose {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin-top: 2px;
  }
  .role-detail {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px 10px;
    background: var(--bg);
    border-radius: var(--radius-sm);
  }
  .role-focus, .role-hint {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .detail-label {
    font-size: 0.7rem;
    font-weight: 600;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }
  .detail-text {
    font-size: 0.8rem;
    color: var(--text-muted);
    line-height: 1.4;
  }
  .role-model {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .model-select-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .model-select-row .detail-label {
    flex-shrink: 0;
    min-width: 45px;
  }
  .model-dropdown {
    flex: 1;
    max-width: 300px;
    font-family: var(--font-mono);
    font-size: 0.8rem;
  }
  .settings-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 0;
    border-bottom: 1px solid var(--border);
  }
  .settings-row:last-child { border-bottom: none; }

  .toggle {
    position: relative;
    display: inline-block;
    width: 36px;
    height: 20px;
    cursor: pointer;
  }
  .toggle input {
    opacity: 0;
    width: 0;
    height: 0;
    position: absolute;
  }
  .toggle-slider {
    position: absolute;
    inset: 0;
    background: var(--bg);
    border-radius: 999px;
    transition: background 0.2s;
  }
  .toggle-slider::before {
    content: '';
    position: absolute;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--text-dim);
    left: 3px;
    top: 3px;
    transition: transform 0.2s, background 0.2s;
  }
  .toggle input:checked + .toggle-slider {
    background: var(--accent-dim);
  }
  .toggle input:checked + .toggle-slider::before {
    transform: translateX(16px);
    background: var(--accent);
  }
</style>
