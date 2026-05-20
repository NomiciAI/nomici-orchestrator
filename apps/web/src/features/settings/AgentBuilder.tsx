import type { FormEvent } from "react";
import type {
  AgentRecord,
  GraphSnapshot,
  ProviderProfile,
  SkillDefinition,
  ToolDefinition,
} from "../../api/types";
import { splitCSV, toggleListValue } from "../../lib/lists";
import { AgentBuilderActions } from "./AgentBuilderActions";
import { AgentSummaryList } from "./AgentSummaryList";

export function AgentBuilder({
  agents,
  models,
  graphSnapshot,
  toolCatalog,
  skillCatalog,
  draft,
  setDraft,
  saving,
  validating,
  validation,
  onValidate,
  onSave,
}: {
  agents: AgentRecord[];
  models: ProviderProfile[];
  graphSnapshot?: GraphSnapshot;
  toolCatalog: ToolDefinition[];
  skillCatalog: SkillDefinition[];
  draft: AgentRecord;
  setDraft: (next: AgentRecord) => void;
  saving: boolean;
  validating: boolean;
  validation: string;
  onValidate: () => void;
  onSave: (event: FormEvent<HTMLFormElement>) => void;
}) {
  const graphModelOptions = graphSnapshot
    ? Object.entries(graphSnapshot.ir.models).map(([id, model]) => ({
        id,
        label: `${id} / ${model.model}`,
      }))
    : [];
  const modelOptions = graphModelOptions.length
    ? graphModelOptions
    : models.map((model) => ({
        id: model.id,
        label: `${model.name || model.id} / ${model.model}`,
      }));
  const agentIsInvalid =
    draft.id.trim() === "" ||
    ((draft.kind === "model_agent" || draft.kind === "gateway_agent") &&
      (draft.model ?? "").trim() === "") ||
    (draft.kind === "external_agent" && (draft.runtime ?? "").trim() === "");
  return (
    <section className="panel" aria-label="Agent builder">
      <div className="panel-heading">
        <div>
          <h2>Agent Builder</h2>
          <p>Create or update shared project agents</p>
        </div>
        <span className="tag">{agents.length}</span>
      </div>
      <form className="builder-form" onSubmit={onSave}>
        <div className="builder-grid">
          <label>
            <span>ID</span>
            <input
              value={draft.id}
              onChange={(event) =>
                setDraft({ ...draft, id: event.target.value })
              }
              placeholder="planner"
            />
          </label>
          <label>
            <span>Name</span>
            <input
              value={draft.name ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, name: event.target.value })
              }
              placeholder="Research Agent"
            />
          </label>
          <label>
            <span>Kind</span>
            <select
              value={draft.kind}
              onChange={(event) =>
                setDraft({ ...draft, kind: event.target.value })
              }
            >
              <option value="model_agent">Model agent</option>
              <option value="gateway_agent">Gateway agent</option>
              <option value="external_agent">External agent</option>
            </select>
          </label>
          <label>
            <span>Model</span>
            <select
              value={draft.model ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, model: event.target.value })
              }
            >
              <option value="">Select model profile</option>
              {modelOptions.map((model) => (
                <option value={model.id} key={model.id}>
                  {model.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>Runtime</span>
            <input
              value={draft.runtime ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, runtime: event.target.value })
              }
              placeholder="local_cli"
            />
          </label>
          <label>
            <span>Approval policy</span>
            <select
              value={draft.approval_policy ?? "default"}
              onChange={(event) =>
                setDraft({ ...draft, approval_policy: event.target.value })
              }
            >
              <option value="default">Default</option>
              <option value="ask">Ask for mutations</option>
              <option value="strict">Strict review</option>
              <option value="readonly">Read-only</option>
            </select>
          </label>
        </div>
        {draft.kind === "external_agent" ? (
          <div className="builder-grid">
            <label>
              <span>Command template</span>
              <input
                value={String(draft.runtime_profile?.command_template ?? "")}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    runtime_profile: {
                      ...(draft.runtime_profile ?? {}),
                      command_template: event.target.value,
                    },
                  })
                }
                placeholder="agent-cli run --prompt {{prompt}}"
              />
            </label>
            <label>
              <span>Timeout seconds</span>
              <input
                type="number"
                min="1"
                max="3600"
                value={String(draft.runtime_profile?.timeout_seconds ?? "")}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    runtime_profile: {
                      ...(draft.runtime_profile ?? {}),
                      timeout_seconds: Number(event.target.value || 0),
                    },
                  })
                }
                placeholder="300"
              />
            </label>
          </div>
        ) : null}
        <div className="builder-grid">
          <label>
            <span>Filesystem permission</span>
            <select
              value={String(draft.permissions?.filesystem ?? "approval")}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  permissions: {
                    ...(draft.permissions ?? {}),
                    filesystem: event.target.value,
                  },
                })
              }
            >
              <option value="read">Read</option>
              <option value="approval">Write with approval</option>
              <option value="none">None</option>
            </select>
          </label>
          <label>
            <span>Bash permission</span>
            <select
              value={String(draft.permissions?.bash ?? "approval")}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  permissions: {
                    ...(draft.permissions ?? {}),
                    bash: event.target.value,
                  },
                })
              }
            >
              <option value="approval">Run with approval</option>
              <option value="none">None</option>
            </select>
          </label>
        </div>
        <label>
          <span>Description</span>
          <input
            value={draft.description ?? ""}
            onChange={(event) =>
              setDraft({ ...draft, description: event.target.value })
            }
            placeholder="When this agent should be selected"
          />
        </label>
        <label>
          <span>Role</span>
          <input
            value={draft.role ?? ""}
            onChange={(event) =>
              setDraft({ ...draft, role: event.target.value })
            }
            placeholder="Plan and coordinate workspace runs"
          />
        </label>
        <label>
          <span>Instructions</span>
          <textarea
            rows={4}
            value={draft.instructions ?? ""}
            onChange={(event) =>
              setDraft({ ...draft, instructions: event.target.value })
            }
            placeholder="Operating instructions"
          />
        </label>
        <div className="builder-grid">
          <label>
            <span>Triggers</span>
            <input
              value={draft.triggers?.join(", ") ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, triggers: splitCSV(event.target.value) })
              }
              placeholder="investigate, implement, verify"
            />
          </label>
          <label>
            <span>Tags</span>
            <input
              value={draft.tags?.join(", ") ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, tags: splitCSV(event.target.value) })
              }
              placeholder="project, local"
            />
          </label>
        </div>
        <div className="selection-panel">
          <div className="mini-heading no-border">
            <strong>Tools</strong>
            <span>{draft.tools?.length ?? 0} selected</span>
          </div>
          <div className="checkbox-grid">
            {toolCatalog.map((tool) => (
              <label key={tool.id}>
                <input
                  type="checkbox"
                  checked={draft.tools?.includes(tool.id) ?? false}
                  onChange={() =>
                    setDraft({
                      ...draft,
                      tools: toggleListValue(draft.tools ?? [], tool.id),
                    })
                  }
                />
                <span>{tool.id}</span>
                <small>{tool.mutation_risk}</small>
              </label>
            ))}
          </div>
        </div>
        <div className="selection-panel">
          <div className="mini-heading no-border">
            <strong>Skills</strong>
            <span>{draft.skills?.length ?? 0} selected</span>
          </div>
          <div className="checkbox-grid">
            {skillCatalog.map((skill) => (
              <label key={skill.id}>
                <input
                  type="checkbox"
                  checked={draft.skills?.includes(skill.id) ?? false}
                  onChange={() =>
                    setDraft({
                      ...draft,
                      skills: toggleListValue(draft.skills ?? [], skill.id),
                    })
                  }
                />
                <span>{skill.name || skill.id}</span>
                <small>{skill.risk || "low"}</small>
              </label>
            ))}
          </div>
        </div>
        <AgentBuilderActions
          validation={validation}
          validating={validating}
          agentIsInvalid={agentIsInvalid}
          saving={saving}
          onValidate={onValidate}
        />
      </form>
      <AgentSummaryList agents={agents} setDraft={setDraft} />
    </section>
  );
}
