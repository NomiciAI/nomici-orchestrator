import { useEffect, useState } from "react";
import type { AgentRecord, OrchestrationConfig, OrchestrationPreview, SkillDefinition, ToolDefinition } from "../../api/types";
import { toggleListValue } from "../../lib/lists";
import { OrchestrationPreviewSummary } from "./OrchestrationPreviewSummary";

export function OrchestrateBuilder({
  agents,
  orchestration,
  toolCatalog,
  skillCatalog,
  preview,
  saving,
  previewing,
  testing,
  onPreview,
  onTest,
  onSave,
}: {
  agents: AgentRecord[];
  orchestration: OrchestrationConfig;
  toolCatalog: ToolDefinition[];
  skillCatalog: SkillDefinition[];
  preview?: OrchestrationPreview | null;
  saving: boolean;
  previewing: boolean;
  testing: boolean;
  onPreview: () => void;
  onTest: () => void;
  onSave: (next: OrchestrationConfig) => void;
}) {
  const modelAgents = agents.filter((agent) => agent.kind !== "tool_agent");
  const [draft, setDraft] = useState<OrchestrationConfig>(orchestration);
  const [selectedRole, setSelectedRole] = useState("");
  const [draggingRole, setDraggingRole] = useState("");
  useEffect(() => {
    setDraft(orchestration);
  }, [orchestration]);
  const roleOrder = draft.role_order?.length
    ? draft.role_order
    : modelAgents.map((agent) => agent.id);
  const disabled = new Set(draft.disabled_roles ?? []);
  const currentRole = selectedRole || roleOrder[0] || "";
  const currentRoleConfig = draft.roles?.[currentRole] ?? {};
  const updateRole = (
    roleID: string,
    patch: NonNullable<OrchestrationConfig["roles"]>[string],
  ) =>
    setDraft({
      ...draft,
      roles: {
        ...(draft.roles ?? {}),
        [roleID]: { ...(draft.roles?.[roleID] ?? {}), ...patch },
      },
    });
  const moveRole = (roleID: string, direction: -1 | 1) => {
    const next = [...roleOrder];
    const index = next.indexOf(roleID);
    const target = index + direction;
    if (index < 0 || target < 0 || target >= next.length) {
      return;
    }
    [next[index], next[target]] = [next[target], next[index]];
    setDraft({ ...draft, role_order: next });
  };
  const dropRole = (targetRoleID: string) => {
    if (!draggingRole || draggingRole === targetRoleID) {
      setDraggingRole("");
      return;
    }
    const next = roleOrder.filter((roleID) => roleID !== draggingRole);
    const targetIndex = next.indexOf(targetRoleID);
    next.splice(targetIndex < 0 ? next.length : targetIndex, 0, draggingRole);
    setDraft({ ...draft, role_order: next });
    setDraggingRole("");
  };
  const normalizedDraft = { ...draft, role_order: roleOrder };
  const hasDiff =
    JSON.stringify(normalizedDraft) !== JSON.stringify(orchestration ?? {});
  return (
    <section className="panel" aria-label="Role flow builder">
      <div className="panel-heading">
        <div>
          <h2>Role flow</h2>
          <p>Sequential MVP flow</p>
        </div>
        <span className="tag">{roleOrder.length}</span>
      </div>
      <div className="builder-grid">
        <label>
          <span>Entrypoint</span>
          <select
            value={draft.entrypoint ?? ""}
            onChange={(event) =>
              setDraft({ ...draft, entrypoint: event.target.value })
            }
            disabled={saving}
          >
            <option value="">Auto</option>
            {agents.map((agent) => (
              <option value={agent.id} key={agent.id}>
                {agent.id}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>Plan review</span>
          <select
            value={draft.plan_review_policy ?? "auto"}
            onChange={(event) =>
              setDraft({
                ...draft,
                plan_review_policy: event.target.value,
              })
            }
            disabled={saving}
          >
            <option value="auto">Auto</option>
            <option value="always">Always</option>
            <option value="never">Never</option>
          </select>
        </label>
      </div>
      <div className="role-library">
        {roleOrder.map((roleID) => (
          <div
            className={`role-toggle ${disabled.has(roleID) ? "role-disabled" : ""} ${
              currentRole === roleID ? "role-selected" : ""
            }`}
            key={roleID}
            draggable
            onDragStart={() => setDraggingRole(roleID)}
            onDragOver={(event) => event.preventDefault()}
            onDrop={() => dropRole(roleID)}
          >
            <button
              type="button"
              disabled={saving}
              onClick={() => setSelectedRole(roleID)}
            >
              <strong>{roleID}</strong>
              <span>{disabled.has(roleID) ? "disabled" : "enabled"}</span>
            </button>
            <div>
              <button
                type="button"
                disabled={saving}
                onClick={() => moveRole(roleID, -1)}
              >
                Up
              </button>
              <button
                type="button"
                disabled={saving}
                onClick={() => moveRole(roleID, 1)}
              >
                Down
              </button>
              <button
                type="button"
                disabled={saving}
                onClick={() => {
                  const nextDisabled = new Set(disabled);
                  if (nextDisabled.has(roleID)) {
                    nextDisabled.delete(roleID);
                  } else {
                    nextDisabled.add(roleID);
                  }
                  setDraft({
                    ...draft,
                    role_order: roleOrder,
                    disabled_roles: [...nextDisabled],
                  });
                }}
              >
                {disabled.has(roleID) ? "Enable" : "Disable"}
              </button>
            </div>
          </div>
        ))}
      </div>
      {currentRole ? (
        <div className="builder-form role-config">
          <div className="mini-heading">
            <strong>{currentRole}</strong>
            <span>Role config</span>
          </div>
          <label>
            <span>Purpose</span>
            <input
              value={currentRoleConfig.purpose ?? ""}
              onChange={(event) =>
                updateRole(currentRole, { purpose: event.target.value })
              }
              placeholder="Role purpose"
            />
          </label>
          <label>
            <span>Instructions</span>
            <textarea
              rows={3}
              value={currentRoleConfig.instructions ?? ""}
              onChange={(event) =>
                updateRole(currentRole, { instructions: event.target.value })
              }
              placeholder="Role-specific operating instructions"
            />
          </label>
          <label>
            <span>Output contract</span>
            <input
              value={currentRoleConfig.output_contract?.description ?? ""}
              onChange={(event) =>
                updateRole(currentRole, {
                  output_contract: {
                    ...(currentRoleConfig.output_contract ?? {}),
                    description: event.target.value,
                  },
                })
              }
              placeholder="Expected deliverable"
            />
          </label>
          <label>
            <span>Role plan review</span>
            <select
              value={currentRoleConfig.plan_review_policy ?? "auto"}
              onChange={(event) =>
                updateRole(currentRole, {
                  plan_review_policy: event.target.value,
                })
              }
              disabled={saving}
            >
              <option value="auto">Auto</option>
              <option value="required">Required</option>
              <option value="disabled">Disabled</option>
            </select>
          </label>
          <div className="selection-panel">
            <div className="mini-heading no-border">
              <strong>Required tools</strong>
              <span>{currentRoleConfig.required_tools?.length ?? 0}</span>
            </div>
            <div className="checkbox-grid">
              {toolCatalog.map((tool) => (
                <label key={tool.id}>
                  <input
                    type="checkbox"
                    checked={
                      currentRoleConfig.required_tools?.includes(tool.id) ??
                      false
                    }
                    onChange={() =>
                      updateRole(currentRole, {
                        required_tools: toggleListValue(
                          currentRoleConfig.required_tools ?? [],
                          tool.id,
                        ),
                      })
                    }
                    disabled={saving}
                  />
                  <span>{tool.id}</span>
                  <small>{tool.mutation_risk}</small>
                </label>
              ))}
            </div>
          </div>
          <div className="selection-panel">
            <div className="mini-heading no-border">
              <strong>Required skills</strong>
              <span>{currentRoleConfig.required_skills?.length ?? 0}</span>
            </div>
            <div className="checkbox-grid">
              {skillCatalog.map((skill) => (
                <label key={skill.id}>
                  <input
                    type="checkbox"
                    checked={
                      currentRoleConfig.required_skills?.includes(skill.id) ??
                      false
                    }
                    onChange={() =>
                      updateRole(currentRole, {
                        required_skills: toggleListValue(
                          currentRoleConfig.required_skills ?? [],
                          skill.id,
                        ),
                      })
                    }
                    disabled={saving}
                  />
                  <span>{skill.name || skill.id}</span>
                  <small>{skill.risk || "low"}</small>
                </label>
              ))}
            </div>
          </div>
        </div>
      ) : null}
      <div className="config-preview">
        <div className="mini-heading">
          <strong>Graph preview</strong>
          <span>{hasDiff ? "changed" : "saved"}</span>
        </div>
        <div className="graph-preview">
          {roleOrder
            .filter((roleID) => !disabled.has(roleID))
            .map((roleID) => draft.roles?.[roleID]?.purpose || roleID)
            .join(" -> ") || "No enabled roles"}
        </div>
        <div className="mini-heading">
          <strong>Pending config</strong>
          <span>{saving ? "saving" : "local draft"}</span>
        </div>
        <pre>{JSON.stringify(draft, null, 2)}</pre>
      </div>
      {preview ? <OrchestrationPreviewSummary preview={preview} /> : null}
      <div className="builder-actions">
        <button
          className="button button-secondary"
          type="button"
          disabled={saving || previewing || testing}
          onClick={onPreview}
        >
          {previewing ? "Previewing" : "Preview flow"}
        </button>
        <button
          className="button button-secondary"
          type="button"
          disabled={saving || previewing || testing}
          onClick={onTest}
        >
          {testing ? "Starting" : "Start test run"}
        </button>
        <button
          className="button"
          type="button"
          disabled={saving}
          onClick={() => onSave({ ...draft, role_order: roleOrder })}
        >
          Save role flow
        </button>
      </div>
    </section>
  );
}
