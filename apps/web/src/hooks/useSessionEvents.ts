import { useEffect } from "react";
import type { TraceEvent } from "../api/types";
import { mergeEvents } from "../lib/format";

type RequestFn = <T>(
  path: string,
  init?: RequestInit,
  tokenOverride?: string,
) => Promise<T>;

type SessionEventsArgs = {
  activeRunId: string;
  activeSessionId: string;
  runEvents: TraceEvent[];
  runStatus: string;
  gatewayToken: string;
  request: RequestFn;
  loadSessionDetail: (sessionId: string) => Promise<void>;
  loadOverview: () => Promise<void>;
  setRunEvents: React.Dispatch<React.SetStateAction<TraceEvent[]>>;
  setRunStatus: React.Dispatch<
    React.SetStateAction<
      "idle" | "starting" | "running" | "completed" | "failed"
    >
  >;
  setRunError: React.Dispatch<React.SetStateAction<string>>;
};

export function useSessionEvents({
  activeRunId,
  activeSessionId,
  runEvents,
  runStatus,
  gatewayToken,
  request,
  loadSessionDetail,
  loadOverview,
  setRunEvents,
  setRunStatus,
  setRunError,
}: SessionEventsArgs) {
  useEffect(() => {
    if (!activeRunId || runStatus !== "running") {
      return;
    }
    const state = { cancelled: false };
    if (activeSessionId) {
      void streamSessionEvents({
        sessionId: activeSessionId,
        state,
        gatewayToken,
        runEvents,
        setRunEvents,
        setRunStatus,
        loadSessionDetail,
        loadOverview,
      });
    }
    const timer = window.setInterval(
      () => {
        void pollRunEvents({
          runId: activeRunId,
          activeSessionId,
          state,
          runEvents,
          request,
          setRunEvents,
          setRunStatus,
          setRunError,
          loadSessionDetail,
          loadOverview,
        });
      },
      activeSessionId ? 2500 : 1200,
    );
    void pollRunEvents({
      runId: activeRunId,
      activeSessionId,
      state,
      runEvents,
      request,
      setRunEvents,
      setRunStatus,
      setRunError,
      loadSessionDetail,
      loadOverview,
    });
    return () => {
      state.cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeRunId, activeSessionId, runEvents, runStatus]);
}

async function pollRunEvents({
  runId,
  activeSessionId,
  state,
  runEvents,
  request,
  setRunEvents,
  setRunStatus,
  setRunError,
  loadSessionDetail,
  loadOverview,
}: {
  runId: string;
  activeSessionId: string;
  state: { cancelled: boolean };
  runEvents: TraceEvent[];
  request: RequestFn;
  setRunEvents: React.Dispatch<React.SetStateAction<TraceEvent[]>>;
  setRunStatus: React.Dispatch<
    React.SetStateAction<
      "idle" | "starting" | "running" | "completed" | "failed"
    >
  >;
  setRunError: React.Dispatch<React.SetStateAction<string>>;
  loadSessionDetail: (sessionId: string) => Promise<void>;
  loadOverview: () => Promise<void>;
}) {
  const lastSequence = runEvents.reduce(
    (max, event) => Math.max(max, event.sequence),
    0,
  );
  try {
    const events = await request<TraceEvent[]>(
      `/api/runs/${encodeURIComponent(runId)}/events?after_sequence=${lastSequence}`,
    );
    if (state.cancelled || events.length === 0) {
      return;
    }
    setRunEvents((current) => mergeEvents(current, events));
    if (activeSessionId) {
      void loadSessionDetail(activeSessionId);
    }
    const terminal = [...events]
      .reverse()
      .find(
        (event) =>
          event.type === "run.session.completed" || event.type === "run.failed",
      );
    if (terminal) {
      setRunStatus(
        terminal.type === "run.session.completed" ? "completed" : "failed",
      );
      void loadOverview();
    }
  } catch (pollError) {
    if (!state.cancelled) {
      setRunError(
        pollError instanceof Error
          ? pollError.message
          : "Run events could not be loaded",
      );
    }
  }
}

async function streamSessionEvents({
  sessionId,
  state,
  gatewayToken,
  runEvents,
  setRunEvents,
  setRunStatus,
  loadSessionDetail,
  loadOverview,
}: {
  sessionId: string;
  state: { cancelled: boolean };
  gatewayToken: string;
  runEvents: TraceEvent[];
  setRunEvents: React.Dispatch<React.SetStateAction<TraceEvent[]>>;
  setRunStatus: React.Dispatch<
    React.SetStateAction<
      "idle" | "starting" | "running" | "completed" | "failed"
    >
  >;
  loadSessionDetail: (sessionId: string) => Promise<void>;
  loadOverview: () => Promise<void>;
}) {
  const lastSequence = runEvents.reduce(
    (max, event) => Math.max(max, event.sequence),
    0,
  );
  const headers: Record<string, string> = { Accept: "text/event-stream" };
  if (gatewayToken.trim() !== "") {
    headers.Authorization = `Bearer ${gatewayToken.trim()}`;
  }
  try {
    const response = await fetch(
      `/api/sessions/${encodeURIComponent(sessionId)}/events?after_sequence=${lastSequence}`,
      { headers },
    );
    if (!response.ok || !response.body) {
      return;
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    while (!state.cancelled) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split("\n\n");
      buffer = parts.pop() ?? "";
      for (const part of parts) {
        const dataLine = part
          .split("\n")
          .find((line) => line.startsWith("data: "));
        if (!dataLine) {
          continue;
        }
        const event = JSON.parse(dataLine.slice(6)) as TraceEvent;
        setRunEvents((current) => mergeEvents(current, [event]));
        void loadSessionDetail(sessionId);
        if (event.type === "run.session.completed") {
          setRunStatus("completed");
          void loadOverview();
        }
      }
    }
  } catch {
    return;
  }
}
