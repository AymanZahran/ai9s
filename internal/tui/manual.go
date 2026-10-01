package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *ui) showManual() {
	tv := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(true)
	tv.SetBorder(true).SetTitle(" manual ")
	tv.SetText(manualText)
	tv.ScrollToBeginning()
	back := func() {
		ui.app.SetRoot(ui.layout, true)
		ui.focusSessions()
	}
	tv.SetDoneFunc(func(tcell.Key) { back() })
	tv.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			back()
			return nil
		}
		if ev.Key() != tcell.KeyRune {
			return ev
		}
		switch ev.Rune() {
		case 'q', '?':
			back()
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case 'g':
			tv.ScrollToBeginning()
			return nil
		case 'G':
			tv.ScrollToEnd()
			return nil
		default:
			return ev
		}
	})
	ui.app.SetRoot(tv, true)
}

const manualText = `[::b]air9s manual[-]

[::b]Views[-]
  [yellow]<1>[-] sessions      the conversation list. Enter resumes. d deletes.
  [yellow]<2>[-] providers     group the current filter by agent
  [yellow]<3>[-] directories   group by working directory
  [yellow]<4>[-] branches      group by git branch
  [yellow]<5>[-] models        group by model

Enter on a group applies that filter and returns to sessions. A group labeled (none) has an empty value, so enter leaves the filter alone.

[::b]Command line[-]
  [yellow]/[-]   edits the filter. Each word is a substring of the title, summary, directory, branch, model, agent, or excerpt.
         Up and down, and page up and page down, move the list while the field is open.
         Left and right stay in the field. j and k are letters here.
  [yellow]:[-]   opens command mode. The table lists views and filter tokens.
         Type to narrow that list. Up and down select a row.
         Enter on an empty command cycles the view. After up or down, Enter applies the highlighted row.
         Another : cycles the view name in the field.
         Esc closes command mode and leaves the filter as it is.

Tokens: agent:  dir:  branch:  model:  date:<7d  date:>30d  date:YYYY-MM-DD  sort:recent|oldest|messages|title
agent:, dir:, branch:, and model: match a substring, so agent:clau finds claude while you type.
A leading ~ in dir: or in free text expands to your home directory.
A token with no value, such as dir:, is ignored until you finish it. Pressing p still drops you into the filter.
Quote a phrase to keep it together: "auth bug".
Esc on the list or in the filter clears the filter. Esc in the preview, the manual, or command mode does not.

[::b]Other keys[-]
  enter       resume in the session directory, or apply a group filter
  tab         focus the preview. j/k scroll a line, ctrl-b/f or page keys scroll a page, g/G jump
  tab, esc    return from the preview to the list
  esc         on the list or in the filter, clear the filter
  d, ctrl-d   delete, after confirmation. Only from the sessions view.
  a           cycle the agent: filter
  p           add a dir: filter
  o           cycle sort
  y           toggle yolo for the next resume
  r           reindex
  s           stats
  ?           this manual
  q           quit
  j / k       move down / up when the list is focused. In / and : they are letters
  up / down   move the list, including while / or : is open

The mouse wheel scrolls the preview while the pointer is over it.

[::b]Resume[-]
Resume runs that agent's own CLI in the session directory when the directory still exists.
  claude --resume
  codex resume
  copilot --resume
  grok --resume
  agy --conversation
  gemini --session-file
  cursor-agent --resume, or agent --resume
  opencode --session
  hermes --resume, with -p for a named profile
  openclaw resume, using the session key
  junie --resume --session-id=
  jules teleport
  goose session --resume --session-id
  cline task open
  aider --restore-chat-history
  kiro-cli chat --resume-id

The Kiro IDE command is a different program. Resume uses kiro-cli.
Goose resume needs the goose command on PATH.

[::b]Yolo[-]
Yolo adds a documented auto-approve flag:
  claude, agy     --dangerously-skip-permissions
  grok            --always-approve
  copilot         --allow-all-tools
  cursor          --force
  hermes          --yolo
  junie           --brave
  cline           --yolo
  kiro-cli        --trust-all-tools
Codex, Gemini, OpenCode, OpenClaw, Jules, Goose, and Aider are resumed without an extra approval flag.

[::b]Delete[-]
Delete removes one session and refuses a path outside that agent's own root.
  claude, codex, cursor    the transcript file, inside that agent's session root
  copilot                  one session-state directory
  agy                      rewrite history.jsonl without that conversation
  opencode                 opencode session delete
  hermes                   hermes sessions delete ID --yes
  openclaw                 openclaw sessions delete KEY --yes
  junie                    the session directory, when it contains transcript.md
  goose                    goose session remove --session-id
  cline                    drop the task from taskHistory.json, then remove its task directory
  aider                    the .aider.chat.history.md file
  grok                     that session directory, plus its active-session and metadata entries
  gemini                   that session-*.json chat file
  jules                    the row in sessions.json or sessions.txt; a cloud session needs JULES_API_KEY
  kiro                     that conversation in data.sqlite3; the shell history table stays
The jules CLI has no delete command. A remote session is deleted through the Jules API when JULES_API_KEY is set.

[::b]Where sessions are read[-]
Each path can be moved with the environment variable after it. A missing directory is skipped.
  claude     CLAUDE_CONFIG_DIR
  codex      CODEX_HOME
  copilot    COPILOT_HOME
  grok       GROK_HOME
  agy        GEMINI_HOME
  gemini     GEMINI_HOME
  cursor     CURSOR_HOME
  opencode   OPENCODE_DB
  hermes     HERMES_HOME, including profiles/<name>/state.db
  openclaw   OPENCLAW_STATE_DIR or OPENCLAW_HOME
  junie      JUNIE_HOME
  jules      JULES_HOME. A remote listing runs only when AIR9S_JULES_REMOTE=1.
  goose      GOOSE_HOME
  cline      CLINE_HOME
  aider      AIDER_CHAT_ROOTS, AIDER_HOME, or AIDER_CHAT_HISTORY. AIDER_SCAN_HOME=1 walks the home directory.
  kiro       KIRO_CLI_DB or KIRO_HOME

The index is $AIR9S_CACHE_DIR/index.db, or $XDG_CACHE_HOME/air9s/index.db, or ~/.cache/air9s/index.db.

[::b]Config[-]
  $AIR9S_CONFIG_DIR, or $XDG_CONFIG_HOME/air9s, or ~/.config/air9s/config.yaml.
  The file sets the mouse, the skin, icons, read-only mode, the starting view, refresh, and plugins.
  Colors live in skins/. Plugins live in plugins.yaml and plugins/.
  air9s info prints the config path, the index path, the skin, and the plugin count.
  A plugin runs a program, not a shell. Core keys always win.
  examples/plugins in the repo has four you can copy into plugins/:
    e  open-editor   open the directory in $AIR9S_EDITOR, or the first editor found
    c  copy-session  copy agent, id, title, and directory to the clipboard
    l  git-story     git status and recent commits ($AIR9S_GIT_LOG, default 20)
    t  new-terminal  a shell in that directory ($AIR9S_TERMINAL, or Terminal.app)
  Enabled plugin keys are drawn on the menu. Restart air9s after adding one.
`
