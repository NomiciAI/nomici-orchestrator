export function AgentBuilderActions({
  validation,
  validating,
  agentIsInvalid,
  saving,
  onValidate,
}: {
  validation: string;
  validating: boolean;
  agentIsInvalid: boolean;
  saving: boolean;
  onValidate: () => void;
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
