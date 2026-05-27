import type { AgentTestResult } from "../../api/types";

export function AgentTestResultPanel({
  result,
}: {
  result: AgentTestResult;
}) {
  const executed = result.mode === "executed";
  return (
    <div className="config-preview">
      <div className="mini-heading">
        <strong>{executed ? "Test result" : "Diagnostic result"}</strong>
        <span>{executed ? "executed" : "not executed"}</span>
      </div>
      {result.truth_label ? (
        <small className="truth-label">{result.truth_label}</small>
      ) : null}
      <p>{result.output || result.mode}</p>
      {result.warnings?.map((warning) => (
        <small key={warning}>{warning}</small>
      ))}
    </div>
  );
}
