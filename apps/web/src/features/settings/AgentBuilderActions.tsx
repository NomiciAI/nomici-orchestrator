export function AgentBuilderActions({
  validation,
  validating,
  testing,
  agentIsInvalid,
  saving,
  onValidate,
  onTest,
}: {
  validation: string;
  validating: boolean;
  testing: boolean;
  agentIsInvalid: boolean;
  saving: boolean;
  onValidate: () => void;
  onTest?: () => void;
}) {
  return (
    <>
      {validation ? (
        <div
          className={validation === "valid" ? "inline-success" : "inline-error"}
        >
          {validation === "valid" ? "Agent config is valid." : validation}
        </div>
      ) : null}
      <button
        className="button button-secondary"
        type="button"
        disabled={validating || agentIsInvalid}
        onClick={onValidate}
      >
        {validating ? "Validating" : "Validate"}
      </button>
      {onTest ? (
        <button
          className="button button-secondary"
          type="button"
          disabled={testing || agentIsInvalid}
          onClick={onTest}
        >
          {testing ? "Testing" : "Test"}
        </button>
      ) : null}
      <button
        className="button"
        type="submit"
        disabled={saving || agentIsInvalid}
      >
        {saving ? "Saving" : "Save agent"}
      </button>
    </>
  );
}
