package report

import (
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"

	"oh-my-gunnsama/remote/internal/observer"
)

func Write(out string, state observer.State) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "state.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(out, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()
	return page.Execute(f, state)
}

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Remote Observer</title><style>
:root{color-scheme:dark;--bg:#07111f;--panel:#101a2e;--ink:#e7f4ff;--muted:#8ba4b8;--accent:#38bdf8;--warn:#f59e0b}
body{margin:0;background:linear-gradient(135deg,#07111f,#102033);color:var(--ink);font-family:Inter,system-ui,sans-serif}main{max-width:1120px;margin:auto;padding:22px}.hero{display:flex;justify-content:space-between;gap:16px;align-items:flex-start}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:14px}.card{background:rgba(16,26,46,.88);border:1px solid #23445f;border-radius:20px;padding:16px}pre{white-space:pre-wrap;color:var(--muted)}.muted{color:var(--muted)}@media(max-width:760px){main{padding:14px}.hero{display:block}}
</style></head><body><main><section class="hero"><div><h1>Remote Observer</h1><p class="muted">Read-only report for {{.ProjectPath}}</p></div><p class="muted">state.json</p></section><section class="grid">{{range .Cards}}<article class="card"><h2>{{.Title}}</h2><pre>{{.Body}}</pre></article>{{end}}</section><section class="card"><h2>Risks</h2>{{if .Risks}}<ul>{{range .Risks}}<li>{{.}}</li>{{end}}</ul>{{else}}<p class="muted">No immediate risks detected.</p>{{end}}</section><section class="card"><h2>Suggested Commands</h2><ul>{{range .SuggestedCommands}}<li><b>{{.Target}}</b>: {{.Text}}</li>{{end}}</ul></section><script>setInterval(()=>fetch('state.json',{cache:'no-store'}).then(()=>location.reload()).catch(()=>{}),5000)</script></main></body></html>`))
