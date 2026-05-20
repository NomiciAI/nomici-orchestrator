import type { GraphSnapshot } from "../api/types";

export type AgentOption = {
  id: string;
  supported: boolean;
  reason: string;
};

export function buildAgentOptions(snapshot?: GraphSnapshot): AgentOption[] {
  if (!snapshot) {
    return [];
  }
  return Object.keys(snapshot.ir.agents)
    .sort()
    .map((id) => {
      const agent = snapshot.ir.agents[id];
      const outgoing = snapshot.ir.edges.filter((edge) => edge.from === id);
      if (agent.kind === "gateway_agent" || agent.kind === "model_agent") {
        if (outgoing.length > 0) {
          return {
            id,
            supported: false,
            reason: "model agents with outgoing edges are not executable yet",
          };
        }
        return {
          id,
          supported: Boolean(agent.model),
          reason: agent.model ? "" : "missing model",
        };
      }
      if (agent.kind !== "external_agent") {
        return {
          id,
          supported: false,
          reason: `${agent.kind} is not executable`,
        };
      }
      if (!agent.runtime) {
        return { id, supported: false, reason: "missing runtime" };
      }
      const runtime = snapshot.ir.runtimes?.[agent.runtime];
      if (!runtime || runtime.kind !== "cli_agent") {
        return { id, supported: false, reason: "runtime is not a cli_agent" };
      }
      if (outgoing.length === 0) {
        return { id, supported: true, reason: "" };
      }
      const chainCheck = checkHandoffChain(snapshot, id);
      return chainCheck.supported
        ? { id, supported: true, reason: "" }
        : { id, supported: false, reason: chainCheck.reason };
    });
}

function checkHandoffChain(
  snapshot: GraphSnapshot,
  startAgentId: string,
): { supported: boolean; reason: string } {
  const visited = new Set<string>([startAgentId]);
  let current = startAgentId;
  for (;;) {
    const outgoing = snapshot.ir.edges.filter((edge) => edge.from === current);
    if (outgoing.length === 0) {
      return { supported: true, reason: "" };
    }
    if (outgoing.length > 1) {
      return {
        supported: false,
        reason: "handoff chain has multiple outgoing edges",
      };
    }
    const edge = outgoing[0];
    if (edge.mode !== "handoff") {
      return { supported: false, reason: "only handoff chains are executable" };
    }
    const target = snapshot.ir.agents[edge.to];
    const targetRuntime = target?.runtime
      ? snapshot.ir.runtimes?.[target.runtime]
      : undefined;
    if (
      target?.kind !== "external_agent" ||
      targetRuntime?.kind !== "cli_agent"
    ) {
      return {
        supported: false,
        reason: "handoff target is not a cli_agent external agent",
      };
    }
    if (visited.has(edge.to)) {
      return { supported: false, reason: "handoff chain contains a cycle" };
    }
    visited.add(edge.to);
    current = edge.to;
  }
}
