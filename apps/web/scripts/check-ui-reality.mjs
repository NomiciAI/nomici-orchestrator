import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const root = new URL("../src", import.meta.url).pathname;
const banned = [
  { pattern: /\bcoming soon\b/i, reason: "Product views must hide unavailable features instead of showing coming soon copy." },
  { pattern: /\bdeferred\b/i, reason: "Deferred roadmap items belong in docs or diagnostics, not product views." },
  { pattern: /\bplaceholder feature\b/i, reason: "Placeholder features must not ship in product views." },
  { pattern: />\s*Helpful\s*</, reason: "Feedback controls are hidden until they are connected to a real review workflow." },
  { pattern: />\s*Not helpful\s*</, reason: "Feedback controls are hidden until they are connected to a real review workflow." },
];

function files(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      return files(path);
    }
    return /\.(tsx?|css)$/.test(path) ? [path] : [];
  });
}

const failures = [];
for (const file of files(root)) {
  const source = readFileSync(file, "utf8");
  for (const check of banned) {
    if (check.pattern.test(source)) {
      failures.push(`${file}: ${check.reason}`);
    }
  }
}

if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
