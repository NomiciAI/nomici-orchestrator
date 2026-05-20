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
        <span>{result.status}</span>
      </div>
      <p>{result.output || result.mode}</p>
      {result.warnings?.map((warning) => (
        <small key={warning}>{warning}</small>
      ))}
    </div>
  );
}
