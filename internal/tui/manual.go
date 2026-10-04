package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *ui) showManual() {
	tv := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(true)
	tv.SetBorder(true).SetTitle(" manual ")
	bg := paintColor(ui.cfg.Skin.Views.Table.Bg, "#000000")
	fg := paintColor(ui.cfg.Skin.Views.Table.Fg, "white")
	tv.SetBackgroundColor(bg)
	tv.SetTextColor(fg)
	tv.SetBorderColor(paintColor(ui.cfg.Skin.Frame.Border.Focus, "white"))
	tv.SetTitleColor(paintColor(ui.cfg.Skin.Frame.Title.Highlight, "white"))
	tv.SetText(manualText)
	tv.ScrollToBeginning()
	ui.manual = tv
	back := func() {
		ui.manual = nil
		ui.app.SetRoot(ui.layout, true)
		ui.restoreBodyFocus()
	}
	tv.SetDoneFunc(func(tcell.Key) { back() })
	tv.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			back()
			return nil
		}
		switch ev.Key() {
		case tcell.KeyUp, tcell.KeyDown:
			if paging(ev) {
				delta := ui.manualPage()
				if ev.Key() == tcell.KeyUp {
					delta = -delta
				}
				ui.scrollManual(delta)
				return nil
			}
		case tcell.KeyPgUp:
			ui.scrollManual(-ui.manualPage())
			return nil
		case tcell.KeyPgDn:
			ui.scrollManual(ui.manualPage())
			return nil
		case tcell.KeyCtrlB:
			ui.scrollManual(-ui.manualPage())
			return nil
		case tcell.KeyCtrlF:
			ui.scrollManual(ui.manualPage())
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

func (ui *ui) manualPage() int {
	if ui.manual == nil {
		return 1
	}
	_, _, _, h := ui.manual.GetInnerRect()
	if h < 1 {
		return 1
	}
	return h
}

func (ui *ui) scrollManual(delta int) {
	if ui.manual == nil || delta == 0 {
		return
	}
	row, col := ui.manual.GetScrollOffset()
	ui.manual.ScrollTo(row+delta, col)
}

const manualText = `[::b]ai9s manual[-]

[::b]Views[-]
  [yellow]<1>[-] sessions      the conversation list. AGE is relative. DATE is the local time. Enter resumes. y resumes with the auto-approve flag. d describes. ctrl-d deletes.
                     Started outside your home directory, the list is that directory and its subdirectories. Home lists every session. Esc clears the filter and keeps that directory.
  [yellow]<2>[-] agents        group the current filter by agent. An agent with no sessions stays listed. Enter on that row says to install its CLI or to log in.
  [yellow]<3>[-] directories   group by working directory
  [yellow]<4>[-] branches      group by git branch and worktree. The worktree is the checkout that holds the session directory. Enter filters by that branch and that checkout.
  [yellow]<5>[-] models        group by model
  [yellow]<6>[-] bookmarks     bookmarked sessions. Enter and y resume. f, n, u, and ctrl-d work here the same way they do on sessions.

Enter on a group applies that filter and returns to sessions. Enter on an agent with no sessions warns you to install that CLI or to log in and start a session. A group labeled (none) has an empty value, so enter leaves the filter alone, unless that branches row still has a worktree.

[::b]Command line[-]
  [yellow]/[-]   edits the filter. Each word matches when its letters appear in order in the name, title, session id, summary, directory, branch, model, agent, or excerpt. A contiguous match ranks above a match with gaps.
         Up and down move the list one row while the field is open. j and k are letters here.
         ⌘↑ and ⌘↓ move a page of rows. ⌘← and ⌘→ move a page of columns. Ctrl or Alt with those arrows do the same.
         Plain left and right stay in the field.
  [yellow]:[-]   opens command mode. The table lists views and filter tokens. agents, or :agents, lists agents. :providers still opens that view. bookmarks, or :bookmarks, lists starred sessions.
         Type to narrow that list. Up and down select a row.
         Enter on an empty command cycles the view. After up or down, Enter applies the highlighted row.
         Another : cycles the view name in the field.
         Esc closes command mode and leaves the filter as it is.

Tokens: agent:  dir:  branch:  model:  mark:yes  mark:no  date:<7d  date:>30d  date:YYYY-MM-DD  sort:recent|oldest|messages|title|cost
agent:, dir:, branch:, and model: match a substring, so agent:clau finds claude while you type.
A leading ~ in dir: or in free text expands to your home directory.
A token with no value, such as dir:, is ignored until you finish it. Pressing p still drops you into the filter.
Quote a phrase to keep it together: "auth bug".
Esc in describe returns to the list and leaves the filter. Esc after you open sessions from a group returns to that group and clears the filter. On that group, Esc clears the filter and stays there. On the sessions list itself, Esc clears the filter. Esc in the manual or in command mode does not.

[::b]Other keys[-]
  enter       resume in the session directory. Quitting the agent returns here. On a group, apply that filter
  y           resume the same way, and pass that agent's auto-approve flag. The top menu shows y yolo on sessions and bookmarks. Describe uses it too. On a group, y says to switch to sessions
  d           describe. The list is replaced by the preview. j/k or up/down scroll a line, h/l or left/right pan, ⌘↑/⌘↓ and ⌘←/⌘→ page, g/G jump
  esc         from describe, return to the list. From a drilled-in list, return to that group. On a group, or on sessions, clear the filter
  ctrl-d      delete, after confirmation. Sessions and bookmarks. d does not delete
  a           cycle the agent: filter
  p           add a dir: filter
  o           cycle sort
  f           bookmark the selected session, or clear that bookmark. A star in the first column marks it. The bookmark stays through a reindex. Sessions and bookmarks. 6 or :bookmarks lists them
  n           rename the selected session. Enter on Save stores it. Esc cancels. An empty name restores the default. The name stays through a reindex. Sessions and bookmarks
  u           usage for the selected session: messages, context, tokens, and the other recorded numbers. Sessions and bookmarks
  s           stats for every indexed session
  ?           this manual. It uses the same black screen as the list. j/k scroll a line. The wheel, page keys, and ⌘↑/⌘↓ scroll a page. g/G jump. q, ?, or esc closes it
  q           quit
  j / k       down / up one line, the same as the down and up arrows, when the list or the preview is focused. In / and : they are letters
  up / down   the same one-line move, including the list while / or : is open
  h / l       pan left / right, the same as the left and right arrows, when the list or the preview is focused. In / and : they are letters
  left / right  the same pan. In / and : they move the cursor in the field, unless Command, Ctrl, or Alt is held
  ⌘↑ / ⌘↓   move a page of rows. The list moves its selection. The preview scrolls its text. Ctrl or Alt with up and down do the same. Page Up and Page Down still do
  ⌘← / ⌘→   move a page of columns, on the list and in the preview, including while / or : is open. Ctrl or Alt with left and right do the same
  ctrl-b / f  the same vertical page motion, in the list and in the preview

The list fills the window. Describe uses that same window until you press esc.
Describe keeps each line intact, so a long line pans sideways instead of wrapping.
AGE is always a relative age. DATE is the local date and time. Sort follows AGE.
NAME is the last column. It is the name set with n. Otherwise it is the session title, or the session id when the session has no title. An empty name restores that default. The name stays through a reindex. sort:title sorts this column. The list has no separate title column. NAME stays whole, and the row pans. Every other column is cut to a fixed width. A path keeps its ending. Describe shows the full value.
The first column is a star when the session is bookmarked. COST is the USD amount the agent recorded, or a dash when the file has none. sort:cost orders by that amount. The list reindexes on the refresh interval. The default is 30 seconds.
Each of those views draws a scrollbar on the right.
When a line is wider than the window, a scrollbar along the bottom pans it.
Hotkeys are on the top menu. The bottom of the screen is empty, except the scrollbar that pans a line wider than the window.
The mouse wheel scrolls the view on screen, including the menu and the crumbs: the list moves several rows, describe scrolls several lines, and this manual scrolls too.
A horizontal wheel pans. Shift with the vertical wheel pans the same way. Drag a scrollbar, or click it, to jump.

[::b]Resume[-]
Resume runs that agent's own CLI in the session directory when the directory still exists.
Enter leaves the agent's permission prompts on. y passes the auto-approve flag when that agent documents one. Codex, Gemini, OpenCode, OpenClaw, Jules, Goose, Aider, Kimi, and MiniMax resume with the same command either way.
Quitting the agent returns to this list. ai9s takes the terminal back, so the shell does not leave it suspended. The index refreshes so the session you just left is current.
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
  kimi --session
  mcode --session
  qwen --resume
  vibe --resume

The Kiro IDE command is a different program. Resume uses kiro-cli.
Kimi resume uses kimi. The archived kimi-cli store is not read.
MiniMax resume uses mcode. Qwen resume uses qwen. Mistral resume uses vibe.
Goose resume needs the goose command on PATH.

[::b]Delete[-]
Delete removes one session and refuses a path outside that agent's own root.
  claude, codex, cursor    the transcript file, inside that agent's session root
  copilot                  one session-state directory
  antigravity             rewrite history.jsonl without that conversation
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
  kimi                     that session directory, and its session_index.jsonl line
  minimax                  that session row and its history directory, or the history directory alone
  qwen                     that chat file
  mistral                  that session directory
The jules CLI has no delete command. A remote session is deleted through the Jules API when JULES_API_KEY is set.

[::b]Where sessions are read[-]
Each path can be moved with the environment variable after it. A missing directory is skipped.
  claude       CLAUDE_CONFIG_DIR
  codex        CODEX_HOME
  copilot      COPILOT_HOME
  grok         GROK_HOME
  antigravity  GEMINI_HOME
  gemini       GEMINI_HOME
  cursor       CURSOR_HOME
  opencode     OPENCODE_DB
  hermes       HERMES_HOME, including profiles/<name>/state.db
  openclaw     OPENCLAW_STATE_DIR or OPENCLAW_HOME
  junie        JUNIE_HOME
  jules        JULES_HOME. A remote listing runs only when AI9S_JULES_REMOTE=1.
  goose        GOOSE_HOME
  cline        CLINE_HOME
  aider        AIDER_CHAT_ROOTS, AIDER_HOME, or AIDER_CHAT_HISTORY. ~/.aider.chat.history.md is read when those are unset. AIDER_SCAN_HOME=1 walks the home directory.
  kiro         KIRO_CLI_DB or KIRO_HOME
  kimi         KIMI_CODE_HOME
  minimax      MINIMAX_DATA_DIR, else MAVIS_DATA_DIR
  qwen         QWEN_RUNTIME_DIR, else QWEN_HOME
  mistral      VIBE_HOME

The index is $AI9S_CACHE_DIR/index.db, or $XDG_CACHE_HOME/ai9s/index.db, or ~/.cache/ai9s/index.db.

[::b]Config[-]
  $AI9S_CONFIG_DIR, or $XDG_CONFIG_HOME/ai9s, or ~/.config/ai9s/config.yaml.
  The file sets the mouse, the skin, icons, read-only mode, the starting view, refresh, and plugins.
  Colors live in skins/. Plugins live in plugins.yaml and plugins/.
  ai9s info prints the config path, the index path, the skin, and the plugin count.
  A plugin runs a program, not a shell. Core keys always win.
  examples/plugins in the repo has four you can copy into plugins/:
    e  open-editor   open the directory in $AI9S_EDITOR, or the first editor found
    c  copy-session  copy agent, id, title, and directory to the clipboard
    b  git-story     git status and recent commits ($AI9S_GIT_LOG, default 20)
    t  new-terminal  a shell in that directory ($AI9S_TERMINAL, or Terminal.app)
  Enabled plugin keys are drawn on the menu. Restart ai9s after adding one.
`
