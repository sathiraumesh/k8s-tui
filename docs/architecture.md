# Architecture

How the code is organised, and how to work on it.

## Layout

```
main.go                     starts the app; prints an error and exits if it fails
internal/
  app/                      creates the cluster client and runs the Bubble Tea program
  kubernetes/               the connection to the cluster
    client.go               Client, NewClient (reads the kubeconfig),
                            NewClientFromClientset (for tests)
    nodes.go                Node, ListNodes, and turning a client-go node into a Node
    resources.go            Resource, ListResources: every resource type (More tab)
  mvc/                      the UI: state, key handling and drawing
    tabs.go                 allTabs: every tab, its loader and whether it auto refreshes
    model.go                Model (all the state), New, Init
    controller.go           Update: what each message and key does
    view.go                 View: draws the screen; styles of tabs, panel, rows
    keys.go                 every key binding, as constants
    messages.go             every message type, and the refresh rates
    nodes_commands.go       loadNodes: the background call for the Nodes tab
    resources_commands.go   loadResources: the background call for the More tab
    refresh.go              the auto refresh timer and rates (logic only)
    secondary_bar_view.go   the buttons on the panel's top border: wide,
                            refresh (drawing only)
    scroll.go               scrolling the list: listHeight, keepCursorVisible
    scroll_view.go          the bottom border: kubectl command, "4-9 of 30"
    border_view.go          withBorderText: writes text into the top or
                            bottom border (shared by both)
  styles/                   the colour palette
examples/                   small stand-alone programs for learning Bubble Tea
                            and Lip Gloss; not part of the app
docs/                       this documentation
```

Dependencies go one way: `main → app → mvc → kubernetes, styles`.
`kubernetes` knows nothing about the UI, and `mvc` never touches client-go
types: it only sees the app's own types, such as `kubernetes.Node`.

## How the UI works

The app follows Bubble Tea's Model / Update / View pattern.

```
          ┌──────────────────────────────────────┐
          ▼                                      │
   Model (state) ──View()──▶ screen              │
                                │                │
                     key press, timer tick,      │
                     data arrives (a message)    │
                                ▼                │
                            Update() ────────────┘
                     (returns the new Model, and maybe
                      a command to run in the background)
```

- **`Model`** ([model.go](../internal/mvc/model.go)) holds everything
  the app remembers: the tabs and their rows, which tab is active, where
  the cursor is, the terminal size, the refresh rate and when each tab
  last loaded.
- **`Update`** ([controller.go](../internal/mvc/controller.go)) is the
  only place the model changes. Key presses go to `onKey`, which looks at
  `m.focus` to decide what the arrow keys do.
- **`View`** ([view.go](../internal/mvc/view.go)) only reads the model
  and returns the screen as a string. Bubble Tea calls it after every
  `Update`. It must never change the model or do slow work.

### Slow work: commands and messages

Calling the cluster can take seconds, so it never happens in `Update` or
`View`. Instead `Update` (or `Init`) returns a **command**: a function
Bubble Tea runs in the background. When it finishes, it returns a
**message**, which Bubble Tea passes to `Update`.

```
Init / Update ──returns──▶ loadNodes(client)        (nodes_commands.go)
                                │ runs in the background: client.ListNodes
                                ▼
                    itemsLoadedMsg{tab, rows, err, at, wide}   (messages.go)
                                │
                                ▼
                   Update stores rows in m.items[tab] ──▶ View draws them
```

Every tab sends the same `itemsLoadedMsg`, so `Update` has one case for
all of them. If the call failed, `m.err` is set and `View` shows it in the
panel; the old rows and update time are kept.

### Auto refresh

[refresh.go](../internal/mvc/refresh.go) runs a timer with `tea.Tick`.
Each `refreshTickMsg` reloads the active tab and schedules the next tick,
so the ticks keep going on their own:

```
Init ──▶ tick(5s) ──▶ refreshTickMsg ──▶ Update ──▶ loadActive + tick(5s) ──▶ ...
```

- The rates are constants in [messages.go](../internal/mvc/messages.go):
  `refreshOff`, `refreshFast` (2s), `refreshNormal` (5s, the default) and
  `refreshSlow` (10s). `refreshRates` is the order the button steps
  through them.
- Setting the rate to `refreshOff` stops scheduling the next tick.
- Every tick carries a generation number, `gen`. Changing the rate starts
  a new timer with a new number, and `Update` drops ticks from older
  timers. Without this, a tick already on its way from the old timer would
  start a second timer, and the tab would reload twice as often.
- `loadActive` runs the active tab's loader from `allTabs`. A tab whose
  `autoRefresh` is false (More) is skipped by the timer; the timer keeps
  running for the other tabs.

### Tabs

Every tab is one entry in `allTabs` ([tabs.go](../internal/mvc/tabs.go)):
its name, its `loader`, its kubectl `command` (shown on the bottom border,
with ` -o wide` added by `kubectlCommand` while the tab is wide), and
whether the timer reloads it. Nothing else in
the app names a particular tab; `loadActive`, the buttons, the update time
and the refresh timer all read this table.

A tab that isn't loaded from the cluster yet (Pods, Deployments, Services)
uses `placeholder(name, rows...)`: a loader that sends the same fixed rows
through the same `itemsLoadedMsg` as a real loader. So it gets an update
time, auto refresh and the wide setting like every other tab, and giving
it real data later is a one-line change in `allTabs`.

### Wide columns

`m.wide` holds each tab's wide setting, keyed by tab name. `toggleWide`
(the `w` key, or the wide button) flips it for the active tab and reloads
the tab. The loader gets the setting (`loadNodes(client, wide)`) and builds
rows with or without the wide columns; `alignColumns` in
[nodes_commands.go](../internal/mvc/nodes_commands.go) pads every column
to its widest cell so they line up.

A load already on its way when wide is switched would arrive with the old
columns. So `itemsLoadedMsg` carries the `wide` it was loaded with, and
`Update` drops rows whose `wide` no longer matches the tab's setting; the
load the switch started brings the right ones.

### Scrolling

The panel shows `listHeight()` rows, starting at `m.offset`
([scroll.go](../internal/mvc/scroll.go)). Whenever the cursor moves, the
window is resized or a tab's rows change, `Update` calls
`keepCursorVisible`, which moves the offset as little as possible so the
cursor's row is on screen and the list is never scrolled past its end.
`View` asks `visibleRange` which rows to draw, and
[scroll_view.go](../internal/mvc/scroll_view.go) writes "4-9 of 30" into
the bottom border while the list is longer than the panel.

Rows are cut to the panel's inner width with `MaxWidth` before they are
drawn: a wrapped row would take two lines and make the panel taller than
the screen.

### Drawing the screen

`View` builds the screen from top to bottom and joins the parts with
`lipgloss.JoinVertical`:

```
tab bar                          1 row
margin                           panelMarginY rows
panel (top border … bottom)      everything that's left
margin                           panelMarginY rows
help line                        1 row
```

The screen must be exactly the terminal's height. If it is taller, the
terminal scrolls and the tab bar disappears off the top. So the panel's
height is worked out from what is left:

```go
// listHeight, in scroll.go: the rows inside the panel
m.height - 2 - 2*panelMarginY - 2 // tab bar + help line, margins, top + bottom border
```

Every number is in terminal cells (characters across, rows down). Lip
Gloss's `Width` and `Height` include padding but not the border or
margin, which is why the border is subtracted by hand.

Lines that could be wider than the terminal (the tab bar and the help
line) are cut off with `MaxWidth(m.width)` instead of wrapping, because a
wrapped line is an extra row.

**The borders.** Lip Gloss can't put text into a border, so the panel is
drawn normally and then its top and bottom border lines are replaced by
`withBorderText` ([border_view.go](../internal/mvc/border_view.go)), with
text on the left and on the right:

```
┌─ [wide] ─ [refresh] ──────── fill ──────── updated 14:30:05 ─┐   withButtons
└─ $ kubectl get nodes ─────── fill ──────────────── 4-9 of 30 ─┘   withCommand
```

`withButtons` ([secondary_bar_view.go](../internal/mvc/secondary_bar_view.go))
and `withCommand` ([scroll_view.go](../internal/mvc/scroll_view.go)) only
choose the text; the drawing is shared. The `─` fill is worked out so the
line is exactly as wide as the box. If the right part doesn't fit it is
left out; if the left doesn't fit either, the plain border stays (and the
cursor skips the buttons, see `buttonsShown`). A command too long for the
border is cut off at its end. The margins are added after both, which need
the borders to be the panel's first and last lines.

### Colours

[styles.go](../internal/styles/styles.go) has the palette, based on
VS Code's Dark Modern theme, plus green and dark gray for buttons that are
on and off. Styles use role names (`Accent`, `Muted`, `On`, `Off`, …)
rather than the raw colours, so the theme can be changed in one place.

## How to…

### Load a tab from the cluster

Using Pods as the example:

1. In `internal/kubernetes`, add a `Pod` type and
   `func (c *Client) ListPods(ctx context.Context) ([]Pod, error)`,
   following [nodes.go](../internal/kubernetes/nodes.go).
2. In `internal/mvc`, add `pods_commands.go` with `loadPods` and `podRows`,
   following [nodes_commands.go](../internal/mvc/nodes_commands.go). It
   returns `itemsLoadedMsg{tab: tabPods, …}`.
3. In `allTabs` ([tabs.go](../internal/mvc/tabs.go)), replace
   `placeholder(tabPods, …)` with `loadPods`.

`Update` needs no change: it already handles `itemsLoadedMsg` for any
tab, and auto refresh, wide and the update time work through `allTabs`.

### Add a tab

Add a name constant and one entry to `allTabs` in
[tabs.go](../internal/mvc/tabs.go), with a loader (`placeholder(...)` to
start with) and `autoRefresh`. The tab bar, buttons and timer pick it up.

### Add a key

1. Add a constant to [keys.go](../internal/mvc/keys.go).
2. Add a `case` to `onKey` in [controller.go](../internal/mvc/controller.go).
   If the key should do different things depending on where the cursor is,
   switch on `m.focus` inside the case.
3. Add it to the help line in [view.go](../internal/mvc/view.go) and to
   [keys.md](keys.md).

### Add a message

1. Define it in [messages.go](../internal/mvc/messages.go).
2. Return it from a command.
3. Add a `case` for it to `Update`.

### Add a button to the top border

1. Add a name constant next to `buttonRefresh` in
   [secondary_bar_view.go](../internal/mvc/secondary_bar_view.go), and append the
   button in `panelButtons` (or a new function that returns the list).
2. Add a `case` for its name to `pressButton` in
   [controller.go](../internal/mvc/controller.go).

`←` / `→` on the buttons, the focus moves and the drawing already work for
any number of buttons.

## Testing

```sh
go test ./...
```

- **`internal/kubernetes`** tests use client-go's fake clientset
  (`k8s.io/client-go/kubernetes/fake`) wrapped with
  `NewClientFromClientset`. To test an API error, add a reactor:
  `cs.PrependReactor("list", "nodes", …)`.
- **`internal/mvc`** tests drive the model directly, with no terminal:
  - `New(nil)` makes a model with no cluster: `Init` and `loadActive`
    return nothing, so no test touches a real cluster.
  - `press(t, m, keys...)` sends messages through `Update` in order.
  - `sized(t, m, w, h)` gives the model a terminal size, so `View` draws
    the full screen.
  - `testModel()` puts three fake nodes on the first tab;
    `onScreen(t)` is the same at 80×24.
  - Tests that run the refresh timer set `m.refreshEvery` to a millisecond
    first, so they don't wait.

Without a terminal, Lip Gloss leaves out colours, so view tests check the
text and the layout (which line is where, and how wide), not colours.
