import {
  emptyOverview,
  type ChatMessage,
  type Overview,
  type RouteDecision,
  type RunTask,
  type Theme,
  type TraceEvent,
  type View,
} from "../api/types";

export function normalizeOverview(next: Overview): Overview {
  return {
    ...emptyOverview,
    ...next,
    gateway: { ...emptyOverview.gateway, ...next.gateway },
    counts: { ...emptyOverview.counts, ...next.counts },
    models: next.models ?? [],
    tools: next.tools ?? [],
    recent_sessions: next.recent_sessions ?? [],
    latest_trace: next.latest_trace ?? [],
    pending_approvals: next.pending_approvals ?? [],
  };
}

export function latestRouteDecision(
  messages: ChatMessage[],
): RouteDecision | null {
  for (const message of [...messages].reverse()) {
    if (message.metadata?.route_decision) {
      return message.metadata.route_decision;
    }
  }
  return null;
}

export function taskTone(status: string): string {
  switch (status) {
    case "completed":
      return "pill-green";
    case "failed":
    case "cancelled":
      return "pill-red";
    case "running":
    case "waiting_for_approval":
    case "plan_review":
      return "pill-amber";
    default:
      return "";
  }
}

export function taskRoleLabel(task: RunTask): string {
  if (task.metadata?.role_id && task.metadata.role_id !== task.agent_id) {
    return `${task.metadata.role_id} / ${task.agent_id}`;
  }
  return task.metadata?.role_id || task.agent_id;
}

export function mergeEvents(
  current: TraceEvent[],
  next: TraceEvent[],
): TraceEvent[] {
  const byID = new Map<string, TraceEvent>();
  for (const event of current) {
    byID.set(event.event_id, event);
  }
  for (const event of next) {
    byID.set(event.event_id, event);
  }
  return [...byID.values()].sort((a, b) => a.sequence - b.sequence);
}

export function eventOutput(event: TraceEvent): string {
  const payload = event.payload ?? {};
  for (const key of [
    "output_preview",
    "stdout_preview",
    "stderr_preview",
    "message",
    "error",
  ]) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") {
      return value;
    }
  }
  return "";
}

export function humanOutput(events: TraceEvent[]): string {
  for (const event of [...events].reverse()) {
    const output = eventOutput(event);
    if (output) {
      return output;
    }
  }
  return "";
}

export function readTheme(): Theme {
  const saved = window.localStorage.getItem("nomici.console.theme");
  if (saved === "light" || saved === "dark") {
    return saved;
  }
  return window.matchMedia("(prefers-color-scheme: light)").matches
    ? "light"
    : "dark";
}

export function formatTime(value: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function viewTitle(view: View): string {
  switch (view) {
    case "chat":
      return "Chat";
    case "orchestrate":
      return "Orchestrate";
    case "settings":
      return "Settings";
    default:
      return "Chat";
  }
}
