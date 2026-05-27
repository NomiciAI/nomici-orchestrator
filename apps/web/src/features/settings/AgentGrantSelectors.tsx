import type { AgentRecord, SkillDefinition, ToolDefinition } from "../../api/types";
import { toggleListValue } from "../../lib/lists";

export function AgentGrantSelectors({
  draft,
  setDraft,
  toolCatalog,
  skillCatalog,
}: {
  draft: AgentRecord;
  setDraft: (next: AgentRecord) => void;
  toolCatalog: ToolDefinition[];
  skillCatalog: SkillDefinition[];
}) {
  const executableTools = toolCatalog.filter(
    (tool) => tool.execution_status !== "unavailable",
  );
  const enabledSkills = skillCatalog.filter((skill) => skill.enabled !== false);
  return (
    <>
      <div className="selection-panel">
        <div className="mini-heading no-border">
          <strong>Tools</strong>
          <span>{draft.tools?.length ?? 0} selected</span>
        </div>
        <div className="checkbox-grid">
          {executableTools.map((tool) => (
            <label key={tool.id}>
              <input
                type="checkbox"
                disabled={tool.execution_status === "configured_only"}
                checked={draft.tools?.includes(tool.id) ?? false}
                onChange={() =>
                  setDraft({
                    ...draft,
                    tools: toggleListValue(draft.tools ?? [], tool.id),
                  })
                }
              />
              <span>{tool.id}</span>
              <small>
                {tool.execution_status === "configured_only"
                  ? "diagnostic"
                  : tool.mutation_risk}
              </small>
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
          {enabledSkills.map((skill) => (
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
    </>
  );
}
