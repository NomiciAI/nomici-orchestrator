import type { AgentTestResult } from "../../api/types";

export function AgentTestResultPanel({
  result,
}: {
  result: AgentTestResult;
}) {
  return (
    <div className="config-preview">
      <div className="mini-heading">
        <strong>Test result</strong>
        <span>{result.mode}</span>
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
