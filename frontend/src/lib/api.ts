// API client for the Warpspawn Go backend

const getToken = (): string => {
  const params = new URLSearchParams(window.location.search);
  const token = params.get('token') || '';
  // Store in sessionStorage so subsequent navigations don't need the query param
  if (token) sessionStorage.setItem('ws_token', token);
  return sessionStorage.getItem('ws_token') || '';
};

const headers = (): HeadersInit => ({
  'Authorization': `Bearer ${getToken()}`,
  'Content-Type': 'application/json',
});

async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const resp = await fetch(path, { ...options, headers: headers() });
  if (!resp.ok) throw new Error(`API error ${resp.status}: ${await resp.text()}`);
  return resp.json();
}

// API endpoints
export const getHealth = () => api<{ status: string; version: string }>('/api/health');
export const getProjects = () => api<ProjectSummary[]>('/api/projects');
export const getBudget = () => api<BudgetInfo>('/api/budget');
export const getSettings = () => api<AppSettings>('/api/settings');
export const updateSettings = (settings: Partial<AppSettings>) =>
  api<AppSettings>('/api/settings', { method: 'PUT', body: JSON.stringify(settings) });
export const testProvider = (provider: string, config: Record<string, string>) =>
  api<{ ok: boolean; error?: string; models?: string[] }>('/api/provider/test', {
    method: 'POST', body: JSON.stringify({ provider, ...config }),
  });
export const createProject = (brief: string, name?: string) =>
  api<{ id: string }>('/api/project/create', {
    method: 'POST', body: JSON.stringify({ brief, name }),
  });
export const startRun = (projectId: string) =>
  api<{ run_id: string }>('/api/run/start', {
    method: 'POST', body: JSON.stringify({ project_id: projectId }),
  });
export const abortRun = (runId: string) =>
  api<void>('/api/run/abort', {
    method: 'POST', body: JSON.stringify({ run_id: runId }),
  });

// SSE event stream
export function connectEvents(onEvent: (event: SSEEvent) => void): EventSource {
  const token = getToken();
  const es = new EventSource(`/api/events?token=${token}`);
  es.onmessage = (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch { /* ignore parse errors */ }
  };
  es.onerror = () => {
    // Auto-reconnect is built into EventSource
  };
  return es;
}

// Types
export interface ProjectSummary {
  ID: string;
  Name: string;
  Lifecycle: string;
  CurrentStage: string;
  TotalTasks: number;
  DoneTasks: number;
}

export interface BudgetInfo {
  daily_cost_usd: number;
  daily_limit_usd: number;
  date: string;
}

export interface AppSettings {
  config_version: number;
  providers: Record<string, ProviderConfig>;
  roles: Record<string, RoleConfig>;
  budget: { daily_limit_usd: number };
  execution: { max_tool_calls: number; agent_timeout_s: number; shell_mode: string };
}

export interface ProviderConfig {
  enabled: boolean;
  base_url?: string;
  key_ref?: string;
}

export interface RoleConfig {
  provider: string;
  model: string;
}

export interface SSEEvent {
  type: string;
  data: unknown;
}
