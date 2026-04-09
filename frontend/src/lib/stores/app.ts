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
export const currentView = writable<'dashboard' | 'project' | 'settings'>('dashboard');
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
export function addNotification(type: Notification['type'], message: string, dismissable = true) {
  notifications.update(n => [...n, { id: ++notifId, type, message, timestamp: Date.now(), dismissable }]);
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

// Handle SSE events
export function handleSSEEvent(event: SSEEvent) {
  const d = event.data as Record<string, unknown>;

  switch (event.type) {
    case 'agent.text':
    case 'agent.chunk':
      if (d?.content) appendLog('text', String(d.content));
      break;
    case 'agent.tool_call':
    case 'agent.tool':
      if (d?.content) appendLog('tool_call', String(d.content));
      break;
    case 'agent.tool_result':
      if (d?.content) appendLog('tool_result', String(d.content));
      break;
    case 'agent.complete':
    case 'agent.error':
      if (d?.summary) appendLog('complete', String(d.summary));
      activeRun.set(null);
      break;
    case 'build.cycle': {
      const cycle = d?.cycle || '?';
      appendLog('text', `\n--- Build cycle ${cycle} ---`);
      break;
    }
    case 'build.progress': {
      const action = d?.action || 'unknown';
      const state = d?.state || '';
      appendLog('text', `Action: ${action} → ${state}`);
      break;
    }
    case 'build.complete':
      appendLog('complete', 'Build finished — all cycles complete');
      activeRun.set(null);
      addNotification('success', 'Build complete');
      break;
    case 'build.cancelled':
      appendLog('error', 'Build cancelled');
      activeRun.set(null);
      break;
    case 'build.budget-exhausted':
      appendLog('error', 'Build paused — daily budget exhausted');
      activeRun.set(null);
      addNotification('warning', 'Budget exhausted — build paused');
      break;
    case 'run.complete':
      // Legacy single-run event
      break;
    case 'escalation':
      addNotification('warning', `Escalation: ${JSON.stringify(d)}`);
      break;
    case 'error':
      addNotification('error', `Error: ${JSON.stringify(d)}`);
      break;
  }
}
