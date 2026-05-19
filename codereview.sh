#!/bin/bash
PORT=${1:-8787}
BASE="/Users/eastsidegunn/Gunnsplayground/oh-my-Gunnsama/reference"

FILES=(
  # OMO Agents
  "oh-my-openagent/src/agents/sisyphus.ts"
  "oh-my-openagent/src/agents/sisyphus/default.ts"
  "oh-my-openagent/src/agents/sisyphus/claude-opus-4-7.ts"
  "oh-my-openagent/src/agents/sisyphus/gpt-5-5.ts"
  "oh-my-openagent/src/agents/sisyphus/AGENTS.md"
  "oh-my-openagent/src/agents/oracle.ts"
  "oh-my-openagent/src/agents/explore.ts"
  "oh-my-openagent/src/agents/librarian.ts"
  "oh-my-openagent/src/agents/hephaestus/agent.ts"
  "oh-my-openagent/src/agents/metis.ts"
  "oh-my-openagent/src/agents/momus.ts"
  "oh-my-openagent/src/agents/dynamic-agent-prompt-builder.ts"
  "oh-my-openagent/src/agents/dynamic-agent-core-sections.ts"
  # OMO Hooks
  "oh-my-openagent/src/hooks/context-window-monitor.ts"
  "oh-my-openagent/src/hooks/model-fallback/hook.ts"
  "oh-my-openagent/src/hooks/model-fallback/fallback-state-controller.ts"
  "oh-my-openagent/src/hooks/ralph-loop/ralph-loop-hook.ts"
  "oh-my-openagent/src/hooks/ralph-loop/loop-state-controller.ts"
  "oh-my-openagent/src/hooks/ralph-loop/completion-promise-detector.ts"
  "oh-my-openagent/src/hooks/ralph-loop/continuation-prompt-injector.ts"
  "oh-my-openagent/src/hooks/ralph-loop/ralph-loop-event-handler.ts"
  "oh-my-openagent/src/hooks/write-existing-file-guard/index.ts"
  "oh-my-openagent/src/hooks/bash-file-read-guard.ts"
  # OMO Features
  "oh-my-openagent/src/features/builtin-commands/commands.ts"
  "oh-my-openagent/src/features/builtin-skills/skills/review-work.ts"
  "oh-my-openagent/src/features/background-agent/manager.ts"
  "oh-my-openagent/src/features/background-agent/spawner.ts"
  "oh-my-openagent/src/features/background-agent/concurrency.ts"
  "oh-my-openagent/src/features/background-agent/state.ts"
  # OMO Tools
  "oh-my-openagent/src/tools/delegate-task/prompt-builder.ts"
  "oh-my-openagent/src/tools/delegate-task/builtin-categories.ts"
  "oh-my-openagent/src/tools/background-task/create-background-task.ts"
  # OMO Config
  "oh-my-openagent/src/plugin-config.ts"
  "oh-my-openagent/src/config/schema/oh-my-opencode-config.ts"
  # OMX Hooks
  "oh-my-codex/src/hooks/codebase-map.ts"
  "oh-my-codex/src/hooks/triage-heuristic.ts"
  "oh-my-codex/src/hooks/keyword-detector.ts"
  "oh-my-codex/src/hooks/keyword-registry.ts"
  # OMX Team
  "oh-my-codex/src/team/orchestrator.ts"
  "oh-my-codex/src/team/contracts.ts"
  # OMX Ralph
  "oh-my-codex/src/ralph/contract.ts"
  # OMX Agents
  "oh-my-codex/src/agents/definitions.ts"
  # OMX Verification
  "oh-my-codex/src/verification/verifier.ts"
)

# Build JSON manifest
echo "["  > /tmp/omg-manifest.json
FIRST=true
for f in "${FILES[@]}"; do
  FULL="$BASE/$f"
  if [ -f "$FULL" ]; then
    if [ "$FIRST" = true ]; then FIRST=false; else echo "," >> /tmp/omg-manifest.json; fi
    echo "\"$f\"" >> /tmp/omg-manifest.json
  fi
done
echo "]" >> /tmp/omg-manifest.json

echo "Serving on http://localhost:$PORT"
echo "Files: $(cat /tmp/omg-manifest.json | python3 -c 'import sys,json;print(len(json.load(sys.stdin)))')"

python3 -c "
import http.server, json, os, urllib.parse

BASE = '$BASE'
PORT = $PORT

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        path = urllib.parse.unquote(self.path)
        if path == '/' or path == '/index.html':
            self.send_response(200)
            self.send_header('Content-Type', 'text/html; charset=utf-8')
            self.end_headers()
            self.wfile.write(HTML.encode())
        elif path == '/manifest.json':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            with open('/tmp/omg-manifest.json') as f:
                self.wfile.write(f.read().encode())
        elif path.startswith('/file/'):
            fp = os.path.join(BASE, path[6:])
            if os.path.isfile(fp):
                self.send_response(200)
                self.send_header('Content-Type', 'text/plain; charset=utf-8')
                self.end_headers()
                with open(fp) as f:
                    self.wfile.write(f.read().encode())
            else:
                self.send_response(404)
                self.end_headers()
        else:
            self.send_response(404)
            self.end_headers()
    def log_message(self, fmt, *args): pass

HTML = '''<!DOCTYPE html>
<html lang=\"ko\">
<head>
<meta charset=\"utf-8\">
<title>OMO/OMX Code Review</title>
<style>
  * { margin:0; padding:0; box-sizing:border-box; }
  body { font-family: \"Pretendard\", -apple-system, system-ui, sans-serif; background: #0d1117; color: #c9d1d9; display:flex; height:100vh; }
  #sidebar { width: 320px; min-width: 320px; background: #161b22; border-right: 1px solid #30363d; overflow-y: auto; padding: 16px 0; }
  #sidebar h2 { font-size: 13px; font-weight: 600; color: #8b949e; padding: 8px 16px; margin-top: 12px; letter-spacing: 0.5px; text-transform: uppercase; }
  #sidebar h2:first-child { margin-top: 0; }
  .file-item { display: block; padding: 6px 16px; font-size: 13px; color: #c9d1d9; cursor: pointer; text-decoration: none; border-left: 3px solid transparent; transition: all 0.15s; letter-spacing: -0.2px; line-height: 1.5; }
  .file-item:hover { background: #1c2128; color: #58a6ff; }
  .file-item.active { background: #1c2128; border-left-color: #58a6ff; color: #58a6ff; font-weight: 500; }
  .file-item .dir { color: #8b949e; font-size: 11px; }
  #content { flex:1; overflow-y: auto; padding: 0; }
  #header { position: sticky; top: 0; background: #161b22; border-bottom: 1px solid #30363d; padding: 12px 24px; z-index: 10; display: flex; align-items: center; gap: 12px; }
  #header .filename { font-size: 14px; font-weight: 600; color: #f0f6fc; font-family: \"SF Mono\", \"Fira Code\", monospace; }
  #header .meta { font-size: 12px; color: #8b949e; }
  #code-wrap { padding: 0; }
  pre { margin: 0; padding: 16px 24px; font-family: \"SF Mono\", \"Fira Code\", \"JetBrains Mono\", \"Cascadia Code\", monospace; font-size: 13.5px; line-height: 1.65; letter-spacing: 0.01em; tab-size: 2; overflow-x: auto; }
  .line { display: flex; min-height: 22px; }
  .line:hover { background: rgba(88,166,255,0.06); }
  .line-no { display: inline-block; width: 50px; text-align: right; padding-right: 16px; color: #484f58; user-select: none; flex-shrink: 0; font-size: 12px; line-height: 1.65; }
  .line-text { white-space: pre; flex: 1; }
  #placeholder { display: flex; align-items: center; justify-content: center; height: 100%; color: #484f58; font-size: 15px; }
  ::-webkit-scrollbar { width: 8px; height: 8px; }
  ::-webkit-scrollbar-track { background: #0d1117; }
  ::-webkit-scrollbar-thumb { background: #30363d; border-radius: 4px; }
  ::-webkit-scrollbar-thumb:hover { background: #484f58; }
  .tag { display: inline-block; font-size: 10px; padding: 1px 6px; border-radius: 3px; margin-left: 6px; font-weight: 500; }
  .tag-agent { background: #1f3d2a; color: #3fb950; }
  .tag-hook { background: #2d1f3d; color: #bc8cff; }
  .tag-feature { background: #3d2e1f; color: #d29922; }
  .tag-tool { background: #1f2d3d; color: #58a6ff; }
  .tag-config { background: #3d1f1f; color: #f47067; }
  .tag-omx { background: #1f3d3d; color: #39d2c0; }
</style>
</head>
<body>
<div id=\"sidebar\"></div>
<div id=\"content\">
  <div id=\"placeholder\">파일을 선택하세요</div>
</div>
<script>
const CATEGORIES = {
  \"OMO Agents\": { tag: \"agent\", prefix: \"oh-my-openagent/src/agents/\" },
  \"OMO Hooks\": { tag: \"hook\", prefix: \"oh-my-openagent/src/hooks/\" },
  \"OMO Features\": { tag: \"feature\", prefix: \"oh-my-openagent/src/features/\" },
  \"OMO Tools\": { tag: \"tool\", prefix: \"oh-my-openagent/src/tools/\" },
  \"OMO Config\": { tag: \"config\", prefix: \"oh-my-openagent/src/\" },
  \"OMX\": { tag: \"omx\", prefix: \"oh-my-codex/\" },
};

function categorize(file) {
  for (const [cat, {prefix}] of Object.entries(CATEGORIES)) {
    if (file.startsWith(prefix)) return cat;
  }
  return \"Other\";
}

function shortName(file) {
  return file.split(\"/\").slice(-2).join(\"/\");
}

async function init() {
  const res = await fetch(\"/manifest.json\");
  const files = await res.json();
  const grouped = {};
  files.forEach(f => {
    const cat = categorize(f);
    if (!grouped[cat]) grouped[cat] = [];
    grouped[cat].push(f);
  });
  const sidebar = document.getElementById(\"sidebar\");
  for (const [cat, items] of Object.entries(grouped)) {
    const h2 = document.createElement(\"h2\");
    h2.textContent = cat;
    sidebar.appendChild(h2);
    const tagClass = CATEGORIES[cat]?.tag || \"config\";
    items.forEach(file => {
      const a = document.createElement(\"a\");
      a.className = \"file-item\";
      a.innerHTML = shortName(file);
      a.onclick = () => loadFile(file, a);
      sidebar.appendChild(a);
    });
  }
}

async function loadFile(file, el) {
  document.querySelectorAll(\".file-item\").forEach(e => e.classList.remove(\"active\"));
  el.classList.add(\"active\");
  const res = await fetch(\"/file/\" + encodeURIComponent(file));
  const text = await res.text();
  const lines = text.split(\"\\n\");
  const content = document.getElementById(\"content\");
  const tagClass = \"tag-\" + (Object.values(CATEGORIES).find(c => file.startsWith(c.prefix))?.tag || \"config\");
  content.innerHTML = \`<div id=\"header\"><span class=\"filename\">\${file.split(\"/\").pop()}</span><span class=\"tag \${tagClass}\">\${file.includes(\"codex\") ? \"OMX\" : \"OMO\"}</span><span class=\"meta\">\${lines.length} lines · \${file}</span></div><div id=\"code-wrap\"><pre>\` +
    lines.map((line, i) =>
      \`<div class=\"line\"><span class=\"line-no\">\${i+1}</span><span class=\"line-text\">\${escapeHtml(line)}</span></div>\`
    ).join(\"\") + \"</pre></div>\";
  content.scrollTop = 0;
}

function escapeHtml(s) {
  return s.replace(/&/g,\"&amp;\").replace(/</g,\"&lt;\").replace(/>/g,\"&gt;\").replace(/\"/g,\"&quot;\");
}

init();
</script>
</body>
</html>'''

http.server.HTTPServer(('127.0.0.1', PORT), Handler).serve_forever()
"
