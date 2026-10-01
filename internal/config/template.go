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

const stockSkinYAML = `# Stock air9s skin. Point ui.skin at "stock", or set AIR9S_SKIN=stock.
# Color values are tcell names (dodgerblue, yellow) or #RRGGBB.
# The name default keeps the terminal's own color.
air9s:
  body:
    fgColor: white
    bgColor: black
    logoColor: "#2A55F4"
  frame:
    border:
      fgColor: dodgerblue
      focusColor: yellow
    menu:
      fgColor: white
      keyColor: dodgerblue
      numKeyColor: aqua
    crumbs:
      fgColor: black
      bgColor: steelblue
      activeColor: white
    title:
      fgColor: aqua
      bgColor: black
      highlightColor: yellow
      counterColor: yellow
      filterColor: orange
  views:
    table:
      fgColor: cadetblue
      bgColor: black
      cursorFgColor: black
      cursorBgColor: cadetblue
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
plugins: {}
`
