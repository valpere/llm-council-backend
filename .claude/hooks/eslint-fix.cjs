const { execFileSync } = require("child_process");
const path = require("path");

const FRONTEND_ROOT = path.resolve(__dirname, "../../frontend");

let data = "";
process.stdin.on("data", (chunk) => (data += chunk));
process.stdin.on("end", () => {
  try {
    const json = JSON.parse(data);
    const filePath =
      json.tool_input?.file_path || json.tool_response?.filePath || "";
    if (!/\.(js|jsx)$/.test(filePath)) return;
    const resolved = path.resolve(filePath);
    if (!resolved.startsWith(FRONTEND_ROOT + path.sep)) return;
    execFileSync(
      "npx",
      ["eslint", "--fix", "--no-warn-ignored", resolved],
      { cwd: FRONTEND_ROOT, stdio: "inherit" }
    );
  } catch (e) {
    // Silently skip — don't block Claude's workflow
  }
});
