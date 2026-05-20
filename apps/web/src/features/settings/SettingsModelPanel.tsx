import type { ConsoleState } from "../../hooks/useChatWorkspace";

export function SettingsModelPanel({ state }: { state: ConsoleState }) {
  return (
    <>
      <section className="panel" aria-label="Providers">
        <div className="panel-heading">
          <h2>Provider catalog</h2>
          <span className="tag">{state.providerCatalog.length}</span>
        </div>
        <div className="table">
          <div className="table-row table-head">
            <span>Provider</span>
            <span>Adapter</span>
            <span>Models</span>
            <span>Ready</span>
          </div>
          {state.providerCatalog.map((provider) => (
            <div className="table-row" key={provider.id}>
              <span>
                <strong>{provider.name}</strong>
                <small>
                  {provider.availability_message || provider.description}
                </small>
              </span>
              <span>{provider.adapter_kind}</span>
              <span>{provider.catalog_mode}</span>
              <span>{provider.available ? "ready" : "not ready"}</span>
            </div>
          ))}
        </div>
      </section>
      <section className="panel" aria-label="Model profiles">
        <div className="panel-heading">
          <h2>Configured models</h2>
          <span className="tag">{state.overview.models.length}</span>
        </div>
        <div className="table">
          <div className="table-row table-head">
            <span>Name</span>
            <span>Kind</span>
            <span>Model</span>
            <span>Secret</span>
          </div>
          {state.overview.models.map((model) => (
            <div className="table-row" key={model.id}>
              <span>{model.name}</span>
              <span>{model.kind}</span>
              <span>{model.model}</span>
              <span>{model.api_key_env || "-"}</span>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}
