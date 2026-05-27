import type { AgentRecord, GraphSnapshot, ProviderProfile } from "../../api/types";

export function modelOptions(
  graphSnapshot: GraphSnapshot | undefined,
  models: ProviderProfile[],
) {
  const graphModels = graphSnapshot
    ? Object.entries(graphSnapshot.ir.models).map(([id, model]) => ({
        id,
        label: `${id} / ${model.model}`,
      }))
    : [];
  return graphModels.length
    ? graphModels
    : models.map((model) => ({
        id: model.id,
        label: `${model.name || model.id} / ${model.model}`,
      }));
}

export function normalizeAgentForCompare(agent: AgentRecord) {
  return {
    id: agent.id,
    name: agent.name ?? "",
    description: agent.description ?? "",
    kind: agent.kind,
    model: agent.model ?? "",
    runtime: agent.runtime ?? "",
    role: agent.role ?? "",
    instructions: agent.instructions ?? "",
    tools: agent.tools ?? [],
    skills: agent.skills ?? [],
    tags: agent.tags ?? [],
    triggers: agent.triggers ?? [],
    approval_policy: agent.approval_policy ?? "",
    permissions: agent.permissions ?? {},
    runtime_profile: agent.runtime_profile ?? {},
  };
}
