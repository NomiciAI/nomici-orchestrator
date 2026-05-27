import type { FormEvent } from "react";
import type {
  AgentRecord,
  AgentTestResult,
  GraphSnapshot,
  ProviderProfile,
  SkillDefinition,
  ToolDefinition,
} from "../../api/types";
import { splitCSV } from "../../lib/lists";
import { AgentBuilderActions } from "./AgentBuilderActions";
import { AgentGrantSelectors } from "./AgentGrantSelectors";
import { AgentSummaryList } from "./AgentSummaryList";
import { AgentTestResultPanel } from "./AgentTestResultPanel";
import { modelOptions, normalizeAgentForCompare } from "./agentCompare";

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
  testResult,
  testing = false,
  canTest = true,
  testUnavailableReason = "",
  onValidate,
  onTest,
  onCopy,
  onSetEnabled,
  onDelete,
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
  testResult?: AgentTestResult | null;
  testing?: boolean;
  canTest?: boolean;
  testUnavailableReason?: string;
  onValidate: () => void;
  onTest?: () => void;
  onCopy?: (agent: AgentRecord) => void;
  onSetEnabled?: (agent: AgentRecord, enabled: boolean) => void;
  onDelete?: (agent: AgentRecord) => void;
  onSave: (event: FormEvent<HTMLFormElement>) => void;
}) {
  const availableModels = modelOptions(graphSnapshot, models);
  const agentIsInvalid =
    draft.id.trim() === "" ||
    ((draft.kind === "model_agent" || draft.kind === "gateway_agent") &&
      (draft.model ?? "").trim() === "") ||
    (draft.kind === "external_agent" && (draft.runtime ?? "").trim() === "");
  const savedAgent = agents.find((agent) => agent.id === draft.id);
  const draftMatchesSaved =
    savedAgent !== undefined &&
    JSON.stringify(normalizeAgentForCompare(savedAgent)) ===
      JSON.stringify(normalizeAgentForCompare(draft));
  const testDisabledReason = !canTest
    ? testUnavailableReason || "Agent execution is not ready"
    : savedAgent
      ? draftMatchesSaved
        ? ""
        : "Save before test"
      : "Save before test";
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
              {availableModels.map((model) => (
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
        <AgentGrantSelectors
          draft={draft}
          setDraft={setDraft}
          toolCatalog={toolCatalog}
          skillCatalog={skillCatalog}
        />
        <AgentBuilderActions
          validation={validation}
          validating={validating}
          testing={testing}
          agentIsInvalid={agentIsInvalid}
          saving={saving}
          testDisabledReason={testDisabledReason}
          onValidate={onValidate}
          onTest={canTest ? onTest : undefined}
        />
        {!canTest && testUnavailableReason ? (
          <div className="inline-warning">{testUnavailableReason}</div>
        ) : null}
        {testResult ? <AgentTestResultPanel result={testResult} /> : null}
      </form>
      <AgentSummaryList
        agents={agents}
        setDraft={setDraft}
        onCopy={onCopy}
        onSetEnabled={onSetEnabled}
        onDelete={onDelete}
      />
    </section>
  );
}
