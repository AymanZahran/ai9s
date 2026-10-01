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
  [yellow]/[-]   edits the filter. Free text matches the title and a short excerpt.
  [yellow]:[-]   opens command mode. The table lists views and filter tokens.
         Type to narrow that list. Enter applies the command.
         Enter on an empty command cycles the view.
         Another : cycles the view name in the field.
         Esc closes command mode.

Tokens: agent:  dir:  branch:  model:  date:<7d  date:>30d  date:YYYY-MM-DD  sort:recent|oldest|messages|title
A token with no value, such as dir:, drops you into the filter so you can finish it.
Quote a phrase to keep it together: "auth bug".

[::b]Other keys[-]
  enter       resume in the session directory, or apply a group filter
  tab         focus the preview. j/k scroll a line, ctrl-b/f or page keys scroll a page, g/G jump
  tab, esc    return from the preview to the list
  d, ctrl-d   delete, after confirmation. Only from the sessions view.
  a           cycle the agent: filter
  p           add a dir: filter
  o           cycle sort
  y           toggle yolo for the next resume
  r           reindex
  s           stats
  ?           this manual
  q           quit
  j / k       move down / up

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
Delete is refused when removing the file air9s can see would leave the agent inconsistent.
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
  grok, gemini, jules, kiro    disabled
Jules sessions live in Google's cloud. Kiro keeps every conversation in one database.

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
`
