# tally

> PostToolUse hook that tracks estimated token usage per session and warns Claude before context degradation.

Claude Code doesn't expose token counts to hooks. tally uses tool output byte sizes as a proxy — accumulating an estimate across every tool call and emitting threshold warnings before the context window fills up, so you can compact at a clean boundary instead of getting cut off mid-task.

## Install

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/AgusRdz/tally/main/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/AgusRdz/tally/main/install.ps1 | iex
```

Both scripts download the binary, add it to `PATH`, and run `tally init` to register the Claude Code hooks automatically.

To override the install directory:

```bash
TALLY_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/AgusRdz/tally/main/install.sh | sh
```

```powershell
$env:TALLY_INSTALL_DIR = "C:\tools\tally"; irm https://raw.githubusercontent.com/AgusRdz/tally/main/install.ps1 | iex
```

### Build from source

```bash
git clone https://github.com/AgusRdz/tally.git
cd tally
make install    # builds and copies to ~/.local/bin (Linux/macOS) or %LOCALAPPDATA%\Programs\tally (Windows)
tally init
```

## Hook registration

`tally init` (run automatically by the install scripts) registers two hooks in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      { "hooks": [{ "type": "command", "command": "/path/to/tally" }] }
    ],
    "PreCompact": [
      { "hooks": [{ "type": "command", "command": "/path/to/tally reset" }] }
    ]
  }
}
```

PostToolUse without a matcher runs on every tool. PreCompact resets the session counter with a post-compaction baseline so the estimate stays accurate after context is compacted.

To remove the hooks:

```bash
tally uninstall
```

## How it works

On every tool call, tally:

1. Reads the `tool_result` bytes from stdin
2. Estimates tokens: `bytes / 4 × tool_weight`
3. Accumulates into the session state file (`~/.cache/tally/session_<ID>.json`)
4. Checks fill percentage: `(baseline + estimated) / max_tokens`
5. Emits a warning if a threshold is crossed — silent otherwise

### Threshold warnings

**Warn threshold** (default 60%) — emitted once:
```
⚠ tally: context ~65% full (est. 65,000/100,000 tokens) | 94 tool calls
Consider wrapping up the current task before the next major operation.
```

**Compact threshold** (default 80%) — repeats every 10 tool calls:
```
⚡ tally: context ~82% full (est. 82,000/100,000 tokens) | 127 tool calls
Recommend: /compact now, at a clean task boundary, to avoid mid-task compaction.
```

tally never blocks — it always responds with `{"action": "continue"}`.

### Baselines

Context is never truly empty at session start — system prompt, CLAUDE.md files, and grounding output all occupy space. tally applies a configurable baseline offset so the fill percentage reflects reality from the first tool call.

| Event | Default baseline |
|---|---|
| New session | 10,000 tokens |
| After compaction (PreCompact reset) | 5,000 tokens |

### Tool weights

Raw bytes overestimate some tools and underestimate others. tally applies per-tool multipliers:

| Tool | Weight | Reason |
|---|---|---|
| Read | 1.5× | File content persists across turns |
| Write | 0.2× | Confirmation only; content was already in context |
| Bash | 1.0× | Measures output as-is (or post-chop if installed) |
| Task | 2.0× | Dense summaries that Claude re-references heavily |
| Edit | 0.3× | Compact diff format |
| default | 1.0× | Everything else |

### Hook ordering with chop

If [chop](https://github.com/AgusRdz/chop) is installed, it must run **before** tally in the PostToolUse chain — chop compresses Bash output before it enters context, and tally measures what actually lands in the window:

```json
"PostToolUse": [
  { "hooks": [{ "type": "command", "command": "/path/to/chop" }] },
  { "hooks": [{ "type": "command", "command": "/path/to/tally" }] }
]
```

tally works correctly without chop — the ordering only matters if both are present.

## CLI

```bash
tally              # PostToolUse hook: read stdin, accumulate, check thresholds
tally reset        # PreCompact hook / manual reset
tally status       # show current session estimate and fill %
tally config show  # show resolved config as YAML
tally init         # install Claude Code hooks
tally uninstall    # remove hooks from ~/.claude/settings.json
tally version
tally help
```

## Config

tally works with zero config. To override defaults, create `~/.config/tally/config.yml`:

```yaml
enabled: true
max_tally_tokens: 100000
warn_threshold_pct: 60
compact_threshold_pct: 80
reminder_interval_calls: 10

baselines:
  session_start: 10000  # tokens occupied at fresh session start
  ctx_restore: 5000     # tokens occupied after compaction

tool_weights:
  Read: 1.5
  Write: 0.2
  Bash: 1.0
  Task: 2.0
  Edit: 0.3
  default: 1.0
```

Set `enabled: false` to disable tally without removing the hook.

## Caveats

- **The estimate is wrong** — it's a proxy, not ground truth. Claude Code does not expose real token counts to hooks. Useful for direction, not precision.
- **The model may compact earlier** — Claude Code's internal compaction can trigger before `compact_threshold` if actual usage grows faster than the estimate. tally is a floor signal, not a ceiling guarantee.
- **Baselines are approximations** — if your CLAUDE.md files are large or grounding output is heavy, increase `session_start` in config.
