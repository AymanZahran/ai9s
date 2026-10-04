# Example plugins

These four are off until you copy them into the config `plugins/` directory. ai9s looks up a bare command name on `PATH`, then beside the yaml file. Each command is a program. The yaml does not go through a shell. Session fields stay in separate arguments.

```sh
cp examples/plugins/open-editor examples/plugins/open-editor.yaml ~/.config/ai9s/plugins/
cp examples/plugins/copy-session examples/plugins/copy-session.yaml ~/.config/ai9s/plugins/
cp examples/plugins/git-story examples/plugins/git-story.yaml ~/.config/ai9s/plugins/
cp examples/plugins/new-terminal examples/plugins/new-terminal.yaml ~/.config/ai9s/plugins/
chmod +x ~/.config/ai9s/plugins/open-editor ~/.config/ai9s/plugins/copy-session \
  ~/.config/ai9s/plugins/git-story ~/.config/ai9s/plugins/new-terminal
```

Restart ai9s. The keys show on the menu. They work on the session list, the bookmarks list, and the directories view.

| Key | File | What it does | What you change |
| --- | --- | --- | --- |
| `e` | `open-editor` | Opens `$CWD` in an editor. A terminal editor uses this screen. A GUI editor returns to the list. | `AI9S_EDITOR`, one program name such as `nvim`, `hx`, `code`, `cursor`, or `zed`. |
| `c` | `copy-session` | Copies the arguments to the clipboard, tab-separated. | The `args` list. One argument is copied as itself. |
| `b` | `git-story` | Shows `git status` and recent commits, then waits for enter. | `AI9S_GIT_LOG`, default 20. |
| `t` | `new-terminal` | Opens a terminal in `$CWD` without resuming the agent. | `AI9S_TERMINAL`, one program name. Empty uses Terminal.app on macOS. |

`AI9S_EDITOR` and `AI9S_TERMINAL` are program names. They are not shell lines. Delete a yaml file, or move it out of `plugins/`, to turn that plugin off.
