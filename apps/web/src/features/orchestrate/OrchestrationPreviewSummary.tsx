import type { OrchestrationPreview } from "../../api/types";

export function OrchestrationPreviewSummary({
  preview,
}: {
  preview: OrchestrationPreview;
}) {
  return (
    <div className="config-preview">
      <div className="mini-heading">
        <strong>{preview.run ? "Test run" : "Flow preview"}</strong>
        <span>{preview.status}</span>
      </div>
      {preview.run ? (
        <p className="builder-note">
          Started real session {preview.run.session_id || preview.run.run_id}.
        </p>
      ) : null}
      <div className="graph-preview">
        {(preview.tasks ?? [])
          .map((task) => task.role_id || task.agent_id)
          .join(" -> ") || "No runnable roles"}
      </div>
      {preview.route_decision?.rationale ? (
        <p className="builder-note">{preview.route_decision.rationale}</p>
      ) : null}
      {preview.warnings?.length ? (
        <div className="builder-warning-list">
          {preview.warnings.map((warning) => (
            <span key={warning}>{warning}</span>
          ))}
        </div>
      ) : null}
    </div>
  );
}
