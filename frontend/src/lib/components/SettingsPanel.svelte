<script lang="ts">
  import { settings, showWizard } from '../stores/app';
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
      <h2>Role → Model Assignment</h2>
      <div class="settings-grid">
        {#each Object.entries($settings.roles) as [role, config]}
          <div class="card">
            <div class="flex justify-between items-center">
              <strong class="text-sm">{role}</strong>
              <span class="badge badge-dim">{config.provider}/{config.model}</span>
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
            <p class="text-xs text-dim">Hard limit on tool invocations per run</p>
          </div>
          <input type="number" value={$settings.execution.max_tool_calls} style="max-width: 80px" />
        </div>
        <div class="settings-row">
          <div>
            <label>Agent timeout (seconds)</label>
            <p class="text-xs text-dim">Max wallclock time per agent run</p>
          </div>
          <input type="number" value={$settings.execution.agent_timeout_s} style="max-width: 80px" />
        </div>
        <div class="settings-row">
          <div>
            <label>Shell execution mode</label>
            <p class="text-xs text-dim">Controls what shell commands agents can run</p>
          </div>
          <select value={$settings.execution.shell_mode} style="max-width: 150px">
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
            <p class="text-xs text-dim">Ollama is free. This only applies to cloud API calls.</p>
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
    max-width: 700px;
  }
  .settings-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .settings-section h2 {
    margin-bottom: 10px;
  }
  .settings-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
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
