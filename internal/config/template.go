package config

const defaultConfigYAML = `# air9s config. Created once; later edits are kept.
# Directory: $AIR9S_CONFIG_DIR, or $XDG_CONFIG_HOME/air9s, or ~/.config/air9s.
air9s:
  # Seconds between automatic reindexes. 0 reindexes only when you press r.
  refreshRate: 0
  # When true, d and ctrl-d do not delete.
  readOnly: false
  # sessions, providers, directories, branches, or models.
  defaultView: sessions
  # When true, ctrl-c does not quit.
  noExitOnCtrlC: false
  ui:
    # The mouse is on unless this is false.
    enableMouse: true
    # Hide the menu, the crumbs, and the logo.
    headless: false
    # Hide the logo beside the menu.
    logoless: false
    # Hide agent icons. The column keeps its width.
    noIcons: false
    # File in skins/, without .yaml. Empty uses the built-in skin.
    # AIR9S_SKIN overrides this.
    skin: ""
    # Maximum rows in the table. The ceiling is 2000.
    limit: 400
`

const stockSkinYAML = `# Stock skin, the same palette as k9s stock.
# Point ui.skin at "stock", or set AIR9S_SKIN=stock.
# An empty ui.skin uses these colors as the built-in theme.
# Color values are tcell names (dodgerblue, fuchsia) or #RRGGBB.
# The name default keeps the terminal's own color.
air9s:
  body:
    fgColor: dodgerblue
    bgColor: black
    logoColor: orange
  frame:
    border:
      fgColor: dodgerblue
      focusColor: aqua
    menu:
      fgColor: white
      keyColor: dodgerblue
      numKeyColor: fuchsia
    crumbs:
      fgColor: black
      bgColor: steelblue
      activeColor: orange
    title:
      fgColor: aqua
      bgColor: black
      highlightColor: fuchsia
      counterColor: papayawhip
      filterColor: steelblue
  views:
    table:
      fgColor: blue
      bgColor: black
      cursorFgColor: black
      cursorBgColor: aqua
      header:
        fgColor: white
        bgColor: black
  # Optional per-agent colors. These override the built-in marks.
  agents: {}
`

const defaultPluginsYAML = `# Plugins run a program on the machine. They do not get a shell.
# List them here, or put one plugin in each file under plugins/.
# Reserved keys are ignored: q / : d y r s ? a p o j k g G 1-5, enter, tab, esc, ctrl-d.
# A selected session provides $ID $NATIVE_ID $AGENT $CWD $TITLE $BRANCH $MODEL.
# $FILTER is the filter line. $NAME is the row name.
# scopes: sessions, providers, directories, branches, models, or all.
# background: true starts the program without leaving the UI, and discards its output.
# Examples live in the air9s repo under examples/plugins. Copy a script and its
# yaml file into this directory to turn one on. A bare command name is looked
# up in PATH, then next to the yaml file.
plugins: {}
`
