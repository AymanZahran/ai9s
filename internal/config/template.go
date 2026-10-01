package config

const defaultConfigYAML = `# air9s config. Created once; later edits are kept.
# Directory: $AIR9S_CONFIG_DIR, or $XDG_CONFIG_HOME/air9s, or ~/.config/air9s.
air9s:
  # Seconds between automatic reindexes. 0 reindexes only when you press r.
  refreshRate: 0
  # When true, ctrl-d does not delete. d still opens describe.
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

const stockSkinYAML = `# Stock skin. Same black screen as the k9s black-and-wtf skin.
# Point ui.skin at "stock", or set AIR9S_SKIN=stock.
# An empty ui.skin uses these colors as the built-in theme.
# Color values are tcell names (white, navajowhite) or #RRGGBB.
# The name default keeps the terminal's own color.
air9s:
  body:
    fgColor: white
    bgColor: black
    logoColor: white
  frame:
    border:
      fgColor: white
      focusColor: white
    menu:
      fgColor: white
      keyColor: white
      numKeyColor: navajowhite
    crumbs:
      fgColor: white
      bgColor: black
      activeColor: white
    title:
      fgColor: white
      bgColor: black
      highlightColor: white
      counterColor: navajowhite
      filterColor: slategray
  views:
    table:
      fgColor: white
      bgColor: black
      cursorFgColor: black
      cursorBgColor: white
      header:
        fgColor: gray
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
