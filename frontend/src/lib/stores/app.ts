import { writable, derived } from 'svelte/store';
import type { ProjectSummary, BudgetInfo, AppSettings, SSEEvent } from '../api';

// Configuration state
export type SetupState = 'unconfigured' | 'partial' | 'configured';

export const settings = writable<AppSettings | null>(null);
export const projects = writable<ProjectSummary[]>([]);
export const budget = writable<BudgetInfo | null>(null);
export const agentLog = writable<AgentLogEntry[]>([]);
export const activeRun = writable<ActiveRun | null>(null);
export const notifications = writable<Notification[]>([]);
export const showWizard = writable(false);
export const currentView = writable<'dashboard' | 'project' | 'settings' | 'help'>('dashboard');
export const selectedProjectId = writable<string | null>(null);

// Derived: setup state
export const setupState = derived(settings, ($settings): SetupState => {
  if (!$settings) return 'unconfigured';
  const providers = $settings.providers || {};
  const hasActiveProvider = Object.values(providers).some(p => p.enabled);
  if (!hasActiveProvider) return 'unconfigured';
  return 'configured';
});

// Derived: can perform actions
export const canAct = derived(setupState, ($state) => $state !== 'unconfigured');

export interface AgentLogEntry {
  id: number;
  type: 'text' | 'tool_call' | 'tool_result' | 'complete' | 'error';
  content: string;
  timestamp: number;
}

export interface ActiveRun {
  runId: string;
  projectId: string;
  role: string;
  startedAt: number;
}

export interface Notification {
  id: number;
  type: 'info' | 'warning' | 'error' | 'success';
  message: string;
  timestamp: number;
  dismissable: boolean;
}

let notifId = 0;
const CRITICAL_TYPES = new Set(['error', 'warning']);

export function addNotification(type: Notification['type'], message: string, dismissable = true) {
  const id = ++notifId;
  notifications.update(n => [...n, { id, type, message, timestamp: Date.now(), dismissable }]);

  // Auto-dismiss after 5s unless critical (error/warning)
  if (!CRITICAL_TYPES.has(type)) {
    setTimeout(() => dismissNotification(id), 5000);
  }
}
export function dismissNotification(id: number) {
  notifications.update(n => n.filter(x => x.id !== id));
}

let logId = 0;
export function appendLog(type: AgentLogEntry['type'], content: string) {
  agentLog.update(log => {
    const entry = { id: ++logId, type, content, timestamp: Date.now() };
    const updated = [...log, entry];
    return updated.slice(-500); // keep last 500 entries
  });
}

// MC chat messages received via SSE during builds
export interface MCMessage {
  project_id: string;
  role: string;
  content: string;
  timestamp: number;
}
export const latestMCMessage = writable<MCMessage | null>(null);

// Handle SSE events — only log meaningful milestones, not raw streaming tokens
export function handleSSEEvent(event: SSEEvent) {
  const d = event.data as Record<string, unknown>;

  switch (event.type) {
    // Agent streaming — DON'T log raw text tokens (they're unreadable character-by-character)
    case 'agent.text':
    case 'agent.chunk':
      // Intentionally not logged — too noisy
      break;
    case 'agent.tool_call':
    case 'agent.tool':
      // Log tool calls with clean formatting
      if (d?.content) {
        try {
          const parsed = JSON.parse(String(d.content));
          if (parsed.name) {
            appendLog('tool_call', `${parsed.name}(${summarizeArgs(parsed.arguments)})`);
          }
        } catch {
          appendLog('tool_call', String(d.content).substring(0, 100));
        }
      }
      break;
    case 'agent.tool_result':
      if (d?.content) {
        const content = String(d.content);
        appendLog('tool_result', content.length > 120 ? content.substring(0, 120) + '...' : content);
      }
      break;
    case 'agent.complete':
      if (d?.summary) {
        const summary = String(d.summary);
        if (summary.length > 5) appendLog('complete', summary.substring(0, 200));
      }
      activeRun.set(null);
      break;
    case 'agent.error':
      if (d?.summary) appendLog('error', String(d.summary));
      activeRun.set(null);
      break;

    // Build milestones — these are the primary log entries
    case 'build.cycle':
      // Don't log every cycle — milestones are more meaningful
      break;
    case 'build.milestone':
      if (d?.milestone) appendLog('text', String(d.milestone));
      break;
    case 'build.complete': {
      const summary = d?.summary ? String(d.summary) : '🏁 Build finished — all tasks processed';
      appendLog('complete', summary);
      activeRun.set(null);
      addNotification('success', 'Build complete');
      break;
    }
    case 'build.cancelled':
      appendLog('error', 'Build cancelled');
      activeRun.set(null);
      break;
    case 'build.budget-exhausted':
      appendLog('error', '⚠️ Build paused — daily budget exhausted');
      activeRun.set(null);
      addNotification('warning', 'Budget exhausted — build paused');
      break;
    case 'mc.message':
      latestMCMessage.set(d as MCMessage);
      break;
    case 'run.complete':
      break;
    case 'escalation':
      addNotification('warning', `Escalation: ${JSON.stringify(d)}`);
      break;
    case 'error':
      addNotification('error', `Error: ${JSON.stringify(d)}`);
      break;
  }
}

function summarizeArgs(args: unknown): string {
  if (!args) return '';
  if (typeof args === 'string') {
    try { args = JSON.parse(args); } catch { return args.substring(0, 60); }
  }
  const obj = args as Record<string, unknown>;
  const parts: string[] = [];
  for (const [k, v] of Object.entries(obj)) {
    if (k === 'content') {
      parts.push(`content: ${String(v).length} chars`);
    } else {
      parts.push(`${k}: ${String(v).substring(0, 40)}`);
    }
  }
  return parts.join(', ');
}
