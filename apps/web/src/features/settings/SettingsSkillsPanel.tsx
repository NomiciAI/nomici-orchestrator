import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { splitCSV } from "../../lib/lists";

export function SettingsSkillsPanel({ state }: { state: ConsoleState }) {
  return (
    <>
      <section className="panel" aria-label="Skill library">
        <div className="panel-heading">
          <div>
            <h2>Skills</h2>
            <p>Reusable instructions injected only when selected or matched</p>
          </div>
          <span className="tag">{state.skillCatalog.length}</span>
        </div>
        <div className="skill-card-list">
          {state.skillCatalog.map((skill) => (
            <div className="skill-card" key={skill.id}>
              <div>
                <strong>{skill.name || skill.id}</strong>
                <span>{skill.description || skill.briefing}</span>
                <small>
                  {skill.source || "project"} / {skill.risk || "low"}
                  {skill.required_tools?.length
                    ? ` / tools ${skill.required_tools.join(", ")}`
                    : ""}
                </small>
              </div>
              <button
                className={`switch ${skill.enabled === false ? "" : "switch-on"}`}
                type="button"
                disabled={state.settingsMutation.startsWith("skill:")}
                onClick={() => void state.toggleSkillEnabled(skill)}
                aria-label={`Toggle ${skill.name || skill.id}`}
              >
                <span />
              </button>
            </div>
          ))}
        </div>
      </section>
      <section className="panel" aria-label="Create skill">
        <div className="panel-heading">
          <h2>Create skill</h2>
          <span className="tag">project</span>
        </div>
        <form className="builder-form" onSubmit={state.saveSkillDraft}>
          <div className="builder-grid">
            <label>
              <span>ID</span>
              <input
                value={state.skillDraft.id}
                onChange={(event) =>
                  state.setSkillDraft({
                    ...state.skillDraft,
                    id: event.target.value,
                  })
                }
                placeholder="repo_inspection"
              />
            </label>
            <label>
              <span>Name</span>
              <input
                value={state.skillDraft.name}
                onChange={(event) =>
                  state.setSkillDraft({
                    ...state.skillDraft,
                    name: event.target.value,
                  })
                }
                placeholder="Repo Inspection"
              />
            </label>
            <label>
              <span>Risk</span>
              <select
                value={state.skillDraft.risk || "low"}
                onChange={(event) =>
                  state.setSkillDraft({
                    ...state.skillDraft,
                    risk: event.target.value,
                  })
                }
              >
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
              </select>
            </label>
            <label>
              <span>Required tools</span>
              <input
                value={state.skillDraft.required_tools?.join(", ") ?? ""}
                onChange={(event) =>
                  state.setSkillDraft({
                    ...state.skillDraft,
                    required_tools: splitCSV(event.target.value),
                  })
                }
                placeholder="read_file, search"
              />
            </label>
          </div>
          <label>
            <span>Description</span>
            <input
              value={state.skillDraft.description}
              onChange={(event) =>
                state.setSkillDraft({
                  ...state.skillDraft,
                  description: event.target.value,
                })
              }
              placeholder="When this skill should be used"
            />
          </label>
          <label>
            <span>Triggers</span>
            <input
              value={state.skillDraft.triggers?.join(", ") ?? ""}
              onChange={(event) =>
                state.setSkillDraft({
                  ...state.skillDraft,
                  triggers: splitCSV(event.target.value),
                })
              }
              placeholder="inspect, audit, repo"
            />
          </label>
          <label>
            <span>Briefing</span>
            <textarea
              rows={5}
              value={state.skillDraft.briefing ?? ""}
              onChange={(event) =>
                state.setSkillDraft({
                  ...state.skillDraft,
                  briefing: event.target.value,
                })
              }
              placeholder="Instructions injected when the skill is selected"
            />
          </label>
          <button
            className="button"
            type="submit"
            disabled={state.settingsMutation === "skill"}
          >
            {state.settingsMutation === "skill" ? "Saving" : "Save skill"}
          </button>
        </form>
      </section>
    </>
  );
}
