<script lang="ts">
</script>

<div class="help">
  <h1>How Warpspawn Works</h1>

  <section class="help-section">
    <h2>Overview</h2>
    <p>Warpspawn is an autonomous software delivery tool. You describe a project, and AI agents build it through a structured pipeline with roles, reviews, and guardrails.</p>

    <div class="pipeline-diagram">
      <div class="pipe-step">
        <span class="pipe-num">1</span>
        <strong>Describe</strong>
        <span class="text-xs text-muted">Write a project brief</span>
      </div>
      <span class="pipe-connector">→</span>
      <div class="pipe-step">
        <span class="pipe-num">2</span>
        <strong>Plan</strong>
        <span class="text-xs text-muted">MC creates task list</span>
      </div>
      <span class="pipe-connector">→</span>
      <div class="pipe-step">
        <span class="pipe-num">3</span>
        <strong>Build</strong>
        <span class="text-xs text-muted">Builder writes code</span>
      </div>
      <span class="pipe-connector">→</span>
      <div class="pipe-step">
        <span class="pipe-num">4</span>
        <strong>Review</strong>
        <span class="text-xs text-muted">QA validates work</span>
      </div>
      <span class="pipe-connector">→</span>
      <div class="pipe-step done">
        <span class="pipe-num">5</span>
        <strong>Done</strong>
        <span class="text-xs text-muted">Files on disk</span>
      </div>
    </div>
  </section>

  <section class="help-section">
    <h2>Quick Start</h2>
    <div class="steps">
      <div class="step-card card">
        <strong>1. Configure a provider</strong>
        <p>Go to Settings. If Ollama is running locally, it's auto-detected. For cloud models, enter your OpenAI or Anthropic API key.</p>
      </div>
      <div class="step-card card">
        <strong>2. Create a project</strong>
        <p>Click "+ New Project" on the Dashboard. Describe what you want to build in plain text. Be specific about tech stack, scope, and constraints.</p>
      </div>
      <div class="step-card card">
        <strong>3. Review the plan</strong>
        <p>Mission Control creates a task list. Review it. Type changes in the chat or click "Start Building" to approve.</p>
      </div>
      <div class="step-card card">
        <strong>4. Watch it build</strong>
        <p>Agents work autonomously through each task. The activity log shows progress. You can pause or change the plan at any time.</p>
      </div>
      <div class="step-card card">
        <strong>5. Find your files</strong>
        <p>Project files are in the "Project Files" section. They persist on disk at <code>~/.local/share/warpspawn/projects/</code>. Open the folder to see the code.</p>
      </div>
    </div>
  </section>

  <section class="help-section">
    <h2>Roles</h2>
    <p class="text-muted mb-2">Each agent has a specific job and boundaries. They can only edit files within their scope.</p>
    <div class="role-cards">
      <div class="card role-help">
        <span class="role-emoji">🎛️</span>
        <div>
          <strong>Mission Control</strong>
          <p class="text-xs text-muted">Orchestrates the pipeline. Decomposes your brief into tasks, prioritizes work, routes between roles, closes completed work. Does NOT write code.</p>
        </div>
      </div>
      <div class="card role-help">
        <span class="role-emoji">🛠️</span>
        <div>
          <strong>Builder</strong>
          <p class="text-xs text-muted">Implements code. Reads the task, creates/modifies files, runs commands, validates the result. The only role that produces code.</p>
        </div>
      </div>
      <div class="card role-help">
        <span class="role-emoji">✅</span>
        <div>
          <strong>Reviewer / QA</strong>
          <p class="text-xs text-muted">Validates completed work against acceptance criteria. Inspects files, checks tests. Approves, rejects, or blocks. Does NOT implement missing work.</p>
        </div>
      </div>
      <div class="card role-help">
        <span class="role-emoji">🏗️</span>
        <div>
          <strong>Architect</strong>
          <p class="text-xs text-muted">Defines technical structure, interfaces, and constraints. Used during shaping, not during build.</p>
        </div>
      </div>
      <div class="card role-help">
        <span class="role-emoji">✏️</span>
        <div>
          <strong>UX Designer</strong>
          <p class="text-xs text-muted">Defines user journeys, flows, and acceptance criteria. Used during shaping.</p>
        </div>
      </div>
    </div>
  </section>

  <section class="help-section">
    <h2>Troubleshooting</h2>
    <details class="card">
      <summary><strong>Builder creates empty files or doesn't write code</strong></summary>
      <p class="mt-2 text-sm text-muted">This is usually a context window issue. Go to Settings → LLM Context Window and ensure it's at least 16,384. Smaller values cause the model to "forget" what it's doing mid-task. Also try a more capable model if available.</p>
    </details>
    <details class="card">
      <summary><strong>Build seems stuck or takes too long</strong></summary>
      <p class="mt-2 text-sm text-muted">Check the Activity Log at the bottom of the project page. If the same error repeats, the model is stuck in a loop — click "Pause Building" and try a different approach. You can also increase the agent timeout in Settings.</p>
    </details>
    <details class="card">
      <summary><strong>Mission Control gives wrong information about files</strong></summary>
      <p class="mt-2 text-sm text-muted">MC knows about project files because they're injected into its context. If the model's context window is too small, it may not see this information. Increase the context size in Settings.</p>
    </details>
    <details class="card">
      <summary><strong>"Test Connection" fails for Ollama</strong></summary>
      <p class="mt-2 text-sm text-muted">Ensure Ollama is running: <code>ollama serve</code>. Check it responds at <code>http://localhost:11434</code>. If using a custom port, update the URL in Settings → Providers.</p>
    </details>
    <details class="card">
      <summary><strong>Where are my project files?</strong></summary>
      <p class="mt-2 text-sm text-muted">All projects live at <code>~/.local/share/warpspawn/projects/</code>. Each project has its own directory with <code>app/</code> for code, <code>tasks/</code> for task files, and <code>docs/</code> for specs. Files persist when Warpspawn is closed.</p>
    </details>
  </section>

  <section class="help-section">
    <h2>FAQ</h2>
    <details class="card">
      <summary><strong>Do files persist when Warpspawn is closed?</strong></summary>
      <p class="mt-2 text-sm text-muted">Yes. All project files are stored on disk. Closing Warpspawn does not delete anything.</p>
    </details>
    <details class="card">
      <summary><strong>Can I use cloud models (OpenAI, Anthropic)?</strong></summary>
      <p class="mt-2 text-sm text-muted">Yes. Add your API key in Settings → Providers. Cloud models produce better results but cost per token. The daily budget limit in Settings controls spending.</p>
    </details>
    <details class="card">
      <summary><strong>Is it free with local models?</strong></summary>
      <p class="mt-2 text-sm text-muted">Yes. Ollama models run locally on your GPU with zero API cost. The budget limit only applies to cloud providers.</p>
    </details>
    <details class="card">
      <summary><strong>What models work best?</strong></summary>
      <p class="mt-2 text-sm text-muted">For local: qwen3:8b for planning, qwen2.5-coder:7b for code generation. For cloud: any model with tool-use support. Larger models produce better results but use more VRAM (local) or cost more (cloud).</p>
    </details>
    <details class="card">
      <summary><strong>What security guardrails are active?</strong></summary>
      <p class="mt-2 text-sm text-muted">Shell command allowlist (restricted mode), path containment (agents can't access files outside the project), role boundaries (agents can only edit files in their scope), budget limits, and error loop detection. See Settings → Guardrails for details.</p>
    </details>
  </section>

  <section class="help-section">
    <h2>About</h2>
    <p class="text-muted">Warpspawn is an open-source project. Apache 2.0 license.</p>
    <p class="text-muted text-sm">Architecture: Go backend + Svelte frontend. Single binary, ~16MB.</p>
  </section>
</div>

<style>
  .help {
    max-width: 720px;
    display: flex;
    flex-direction: column;
    gap: 24px;
  }
  .help-section h2 {
    margin-bottom: 8px;
  }
  .pipeline-diagram {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 16px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow-x: auto;
    margin-top: 12px;
  }
  .pipe-step {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    padding: 10px 14px;
    background: var(--bg-elevated);
    border-radius: var(--radius-sm);
    min-width: 80px;
    text-align: center;
  }
  .pipe-step.done {
    background: var(--accent-dim);
  }
  .pipe-num {
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background: var(--bg);
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-muted);
  }
  .pipe-step.done .pipe-num {
    background: var(--accent-dim);
    color: var(--green);
  }
  .pipe-connector {
    color: var(--text-dim);
    font-size: 1.2rem;
    flex-shrink: 0;
  }
  .steps {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .step-card {
    padding: 12px;
  }
  .step-card p {
    margin-top: 4px;
    font-size: 0.85rem;
  }
  .role-cards {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .role-help {
    display: flex;
    gap: 10px;
    padding: 12px;
  }
  .role-emoji {
    font-size: 1.3rem;
    flex-shrink: 0;
  }
  details summary {
    cursor: pointer;
    padding: 10px 0;
  }
  details {
    padding: 0 14px;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.8em;
    background: var(--bg);
    padding: 1px 4px;
    border-radius: 3px;
  }
  .mt-2 { margin-top: 8px; }
  .mb-2 { margin-bottom: 8px; }
</style>
