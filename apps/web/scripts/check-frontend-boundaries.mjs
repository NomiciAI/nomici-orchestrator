import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const root = new URL("..", import.meta.url).pathname;
const src = join(root, "src");

const requiredFiles = [
  "src/api/types.ts",
  "src/api/client.ts",
  "src/hooks/useConsoleState.ts",
  "src/hooks/useChatWorkspace.ts",
  "src/hooks/useSessionEvents.ts",
  "src/features/chat/ChatPage.tsx",
  "src/features/workspace/WorkspacePanel.tsx",
  "src/features/orchestrate/OrchestratePage.tsx",
  "src/features/settings/SettingsPage.tsx",
  "src/lib/format.ts",
  "src/lib/lists.ts",
  "src/styles/index.css",
];

const failures = [];

for (const file of requiredFiles) {
  if (!existsSync(join(root, file))) {
    failures.push(`Missing expected frontend boundary file: ${file}`);
  }
}

if (existsSync(join(src, "styles.css"))) {
  failures.push("src/styles.css must not be restored as the main style entry.");
}

checkLineLimit(join(src, "App.tsx"), 250);

for (const file of walk(join(src, "features"))) {
  if (file.endsWith(".tsx")) {
    checkLineLimit(file, 350);
  }
}

const appSource = readFileSync(join(src, "App.tsx"), "utf8");
if (/type\s+[A-Z][A-Za-z0-9]+\s*=/.test(appSource)) {
  failures.push("App.tsx should not define Gateway DTO/domain types.");
}

if (failures.length > 0) {
  console.error(failures.join("\n"));
  process.exit(1);
}

function checkLineLimit(file, limit) {
  const lines = readFileSync(file, "utf8").split("\n").length;
  if (lines > limit) {
    failures.push(`${relative(root, file)} has ${lines} lines; limit is ${limit}.`);
  }
}

function* walk(dir) {
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry);
    if (statSync(fullPath).isDirectory()) {
      yield* walk(fullPath);
    } else {
      yield fullPath;
    }
  }
}
