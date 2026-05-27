import { useState } from "react";
import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { SettingsMemoryPanel } from "./SettingsMemoryPanel";
import { SettingsModelPanel } from "./SettingsModelPanel";
import { SettingsSkillsPanel } from "./SettingsSkillsPanel";
import { SettingsToolsPanel } from "./SettingsToolsPanel";
import { SettingsDiagnosticsPanel } from "./SettingsDiagnosticsPanel";

type SettingsSection = "models" | "tools" | "skills" | "memory" | "diagnostics";

const sections: Array<{ id: SettingsSection; label: string; hint: string }> = [
  { id: "models", label: "Models", hint: "Providers and profiles" },
  { id: "tools", label: "Tools", hint: "Broker contracts" },
  { id: "skills", label: "Skills", hint: "Reusable instructions" },
  { id: "memory", label: "Memory", hint: "Reusable context" },
  { id: "diagnostics", label: "Diagnostics", hint: "Readiness gates" },
];

export function SettingsPage({ state }: { state: ConsoleState }) {
  const [section, setSection] = useState<SettingsSection>("models");
  return (
    <section className="settings-layout" aria-label="Settings">
      <aside className="settings-nav">
        <div>
          <h2>Settings</h2>
          <p>Configure the harness without editing project files by hand.</p>
        </div>
        {sections.map((item) => (
          <button
            className={section === item.id ? "settings-tab-active" : ""}
            key={item.id}
            type="button"
            onClick={() => setSection(item.id)}
          >
            <strong>{item.label}</strong>
            <span>{item.hint}</span>
          </button>
        ))}
      </aside>
      <div className="settings-content">
        {section === "models" ? <SettingsModelPanel state={state} /> : null}
        {section === "tools" ? <SettingsToolsPanel state={state} /> : null}
        {section === "skills" ? <SettingsSkillsPanel state={state} /> : null}
        {section === "memory" ? <SettingsMemoryPanel state={state} /> : null}
        {section === "diagnostics" ? (
          <SettingsDiagnosticsPanel state={state} />
        ) : null}
      </div>
    </section>
  );
}
