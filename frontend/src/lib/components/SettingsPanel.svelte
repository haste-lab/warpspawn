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

  async function handleProviderToggle(name: string, enabled: boolean) {
    if (!$settings) return;
    const updated = { ...$settings };
    updated.providers = { ...updated.providers };
    updated.providers[name] = { ...updated.providers[name], enabled };
    if (await saveSettings(updated)) {
      addNotification('success', `${name} ${enabled ? 'enabled' : 'disabled'}`);
    }
  }

  async function saveSettings(updated: typeof $settings) {
    if (!updated) return;
    saving = true;
    try {
      const result = await updateSettings(updated);
      settings.set(result);
      return true;
    } catch (e: any) {
      addNotification('error', `Failed to save: ${e.message}`);
      return false;
    } finally {
      saving = false;
    }
  }

  async function handleExecutionChange(field: string, value: string | number) {
    if (!$settings) return;
    const updated = { ...$settings };
    updated.execution = { ...updated.execution, [field]: value };
    if (await saveSettings(updated)) {
      addNotification('success', `Updated ${field.replace(/_/g, ' ')}`);
    }
  }

  async function handleBudgetChange(value: number) {
    if (!$settings || value < 0) return;
    const updated = { ...$settings };
    updated.budget = { ...updated.budget, daily_limit_usd: value };
    if (await saveSettings(updated)) {
      addNotification('success', `Updated daily budget limit to $${value.toFixed(2)}`);
    }
  }

  async function handleRoleModelChange(role: string, value: string) {
    if (!$settings) return;
    const slashIdx = value.indexOf('/');
    if (slashIdx < 0) return;
    const provider = value.substring(0, slashIdx);
    const model = value.substring(slashIdx + 1);

    const updated = { ...$settings };
    updated.roles = { ...updated.roles };
    updated.roles[role] = { provider, model };

    if (await saveSettings(updated)) {
      addNotification('success', `Updated ${role} → ${provider}/${model}`);
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
                <input type="checkbox" checked={config.enabled} on:change={() => handleProviderToggle(name, !config.enabled)} />
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
        <h2>Role → Model Assignment Defaults</h2>
        <p class="text-muted text-sm">These defaults apply to every new project. Individual projects can override them at creation time (via strategy presets) or in project settings. Code-generating roles benefit from capable models, while planning and review roles can use lighter, cheaper ones.</p>
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
      <h2>Guardrails</h2>
      <p class="text-muted text-sm mb-2">Security controls that protect your machine from unintended agent actions.</p>

      <div class="card">
        <div class="settings-row">
          <div>
            <label>Shell execution mode</label>
            <p class="text-xs text-dim">Controls what shell commands agents can run on your machine.</p>
          </div>
          <select value={$settings.execution.shell_mode} style="max-width: 160px"
            on:change={(e) => handleExecutionChange('shell_mode', e.currentTarget.value)}>
            <option value="restricted">Restricted (recommended)</option>
            <option value="unrestricted">Unrestricted</option>
            <option value="approval">Approval required</option>
          </select>
        </div>
      </div>

      <div class="guardrail-grid">
        <div class="guardrail-card card">
          <div class="guardrail-header">
            <span class="guardrail-icon">🛡️</span>
            <strong>Shell Allowlist</strong>
            <span class="badge" class:badge-green={$settings.execution.shell_mode === 'restricted'} class:badge-red={$settings.execution.shell_mode === 'unrestricted'} class:badge-amber={$settings.execution.shell_mode === 'approval'}>
              {$settings.execution.shell_mode}
            </span>
          </div>
          {#if $settings.execution.shell_mode === 'restricted'}
            <p class="text-xs text-muted mt-2">Only these commands are allowed:</p>
            <div class="guardrail-list">
              <span class="mono text-xs">node npm npx python python3 pip go cargo rustc make git ls cat head tail mkdir cp mv touch echo test wc sort uniq grep find chmod rm sh bash</span>
            </div>
            <p class="text-xs text-muted mt-2">Shell bypass protection active — <code>bash -c</code> payloads are parsed and checked against the allowlist.</p>
          {:else if $settings.execution.shell_mode === 'unrestricted'}
            <p class="text-xs text-amber mt-2">All commands allowed. Dangerous patterns still blocked (rm -rf /, sudo, fork bombs).</p>
          {:else}
            <p class="text-xs text-red mt-2">All commands blocked. Agents cannot execute shell commands.</p>
          {/if}
        </div>

        <div class="guardrail-card card">
          <div class="guardrail-header">
            <span class="guardrail-icon">🚫</span>
            <strong>Always Blocked</strong>
            <span class="badge badge-green">Active</span>
          </div>
          <p class="text-xs text-muted mt-2">These are blocked in ALL modes, including unrestricted:</p>
          <div class="guardrail-list">
            <span class="mono text-xs">rm -rf / &bull; sudo &bull; su &bull; chmod 777 / &bull; mkfs &bull; dd if= &bull; fork bombs</span>
          </div>
          <p class="text-xs text-muted mt-2">Network commands blocked in restricted mode:</p>
          <div class="guardrail-list">
            <span class="mono text-xs">curl &bull; wget &bull; ssh &bull; scp &bull; nc &bull; ncat &bull; netcat</span>
          </div>
        </div>

        <div class="guardrail-card card">
          <div class="guardrail-header">
            <span class="guardrail-icon">📁</span>
            <strong>Path Containment</strong>
            <span class="badge badge-green">Active</span>
          </div>
          <p class="text-xs text-muted mt-2">Agents can only read and write files inside the project directory. Path traversal (../) is blocked.</p>
        </div>

        <div class="guardrail-card card">
          <div class="guardrail-header">
            <span class="guardrail-icon">🔒</span>
            <strong>Role Boundaries</strong>
            <span class="badge badge-green">Active</span>
          </div>
          <p class="text-xs text-muted mt-2">Each role has file edit restrictions. After every agent run, changes are validated against the role's allowed paths. Unauthorized files are reverted.</p>
        </div>

        <div class="guardrail-card card">
          <div class="guardrail-header">
            <span class="guardrail-icon">💰</span>
            <strong>Budget Enforcement</strong>
            <span class="badge badge-green">Active</span>
          </div>
          <p class="text-xs text-muted mt-2">Token usage tracked per API call. Execution halts when daily budget limit is reached. Ollama calls are free.</p>
        </div>

        <div class="guardrail-card card">
          <div class="guardrail-header">
            <span class="guardrail-icon">⏱️</span>
            <strong>Timeout Protection</strong>
            <span class="badge badge-green">Active</span>
          </div>
          <p class="text-xs text-muted mt-2">Agent wallclock timeout: {$settings.execution.agent_timeout_s}s. Per-command timeout: 30s. Tool call limit: {$settings.execution.max_tool_calls} calls per run.</p>
        </div>

        <div class="guardrail-card card">
          <div class="guardrail-header">
            <span class="guardrail-icon">🔄</span>
            <strong>Git Rollback</strong>
            <span class="badge badge-green">Active</span>
          </div>
          <p class="text-xs text-muted mt-2">Auto-commit before and after every agent run. If an agent produces bad output, changes can be reverted via git.</p>
        </div>
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
          <input type="number" value={$settings.execution.max_tool_calls} style="max-width: 80px"
            on:change={(e) => handleExecutionChange('max_tool_calls', parseInt(e.currentTarget.value))} />
        </div>
        <div class="settings-row">
          <div>
            <label>Agent timeout (seconds)</label>
            <p class="text-xs text-dim">Max wallclock time per agent run. Prevents runaway token consumption.</p>
          </div>
          <input type="number" value={$settings.execution.agent_timeout_s} style="max-width: 80px"
            on:change={(e) => handleExecutionChange('agent_timeout_s', parseInt(e.currentTarget.value))} />
        </div>
      </div>
    </section>

    <section class="settings-section">
      <h2>LLM Context Window</h2>
      <p class="text-muted text-sm mb-2">Controls how much text the model can process at once. Larger context = better results but more VRAM usage. This only affects local Ollama models — cloud providers manage context automatically.</p>
      <div class="card">
        <div class="settings-row">
          <div>
            <label>Context size (tokens)</label>
            <p class="text-xs text-dim">Minimum 16K for quality output. Larger = better results but more VRAM.</p>
          </div>
          <select value={$settings.execution.llm_context_size || 16384} style="max-width: 140px"
            on:change={(e) => handleExecutionChange('llm_context_size', parseInt(e.currentTarget.value))}>
            <option value="16384">16,384 (8GB VRAM)</option>
            <option value="32768">32,768 (16GB VRAM)</option>
            <option value="65536">65,536 (24GB+ VRAM)</option>
            <option value="131072">131,072 (32GB+ VRAM)</option>
          </select>
        </div>
      </div>
      <div class="hint-card card">
        <p class="text-xs text-muted"><strong>Why this matters:</strong> If context is too small, the model can't see the full task description and forgets what it already did. This leads to empty files, repeated errors, and hallucinated responses. If you see poor build results, try increasing this value.</p>
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
            <input type="number" value={$settings.budget.daily_limit_usd} min="0.5" step="0.5" style="max-width: 80px"
              on:change={(e) => handleBudgetChange(parseFloat(e.currentTarget.value))} />
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
  .guardrail-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 8px;
    margin-top: 10px;
  }
  .guardrail-card {
    padding: 12px;
  }
  .guardrail-header {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.9rem;
  }
  .guardrail-icon {
    font-size: 1rem;
  }
  .guardrail-list {
    padding: 6px 10px;
    background: var(--bg);
    border-radius: var(--radius-sm);
    margin-top: 4px;
    line-height: 1.6;
    word-break: break-all;
  }
  .hint-card {
    background: var(--blue-dim);
    border-color: rgba(90, 182, 255, 0.1);
  }
  .text-amber { color: var(--amber); }
  .text-red { color: var(--red); }
  code {
    font-family: var(--font-mono);
    font-size: 0.8em;
    background: var(--bg);
    padding: 1px 4px;
    border-radius: 3px;
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
