import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

const rootElement = document.getElementById("root");

function showBootFailure(error: unknown) {
  if (!rootElement || rootElement.querySelector(".shell")) {
    return;
  }
  const message =
    error instanceof Error
      ? `${error.name}: ${error.message}`
      : String(error || "Unknown boot error");
  rootElement.replaceChildren();
  const screen = document.createElement("section");
  screen.className = "boot-screen";
  const card = document.createElement("div");
  card.className = "boot-card";
  const title = document.createElement("strong");
  title.textContent = "Nomici Console could not start";
  const body = document.createElement("span");
  body.textContent =
    "The local Gateway is reachable, but the Console JavaScript failed before React mounted.";
  const command = document.createElement("code");
  command.textContent =
    "scripts/install.sh --from-source . && nomici gateway open";
  const details = document.createElement("pre");
  details.textContent = message;
  card.append(title, body, command, details);
  screen.append(card);
  rootElement.append(screen);
}

window.addEventListener("error", (event) => showBootFailure(event.error));
window.addEventListener("unhandledrejection", (event) =>
  showBootFailure(event.reason),
);

async function boot() {
  if (!rootElement) {
    throw new Error("Missing #root element");
  }
  try {
    const { App } = await import("./App");
    createRoot(rootElement).render(
      <StrictMode>
        <App />
      </StrictMode>,
    );
  } catch (error) {
    showBootFailure(error);
  }
}

void boot();
