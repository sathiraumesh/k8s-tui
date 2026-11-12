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
  mvc/                      the UI: state, key handling and drawing
    model.go                Model (all the state), New, Init
    controller.go           Update: what each message and key does
    view.go                 View: draws the screen; styles of tabs, panel, rows
    keys.go                 every key binding, as constants
    messages.go             every message type, and the refresh rates
    nodes_commands.go       loadNodes: the background call for the Nodes tab
    refresh.go              the auto refresh timer and rates (logic only)
    refresh_view.go         the refresh button and the top border (drawing only)
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
                    itemsLoadedMsg{tab, rows, err, at}   (messages.go)
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
- `loadActive` picks the loader for the active tab. A tab with no loader
  yet keeps its placeholder rows; the timer keeps running.

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
bodyH := m.height - 2 - 2*panelMarginY // tab bar + help line + margins
panelH := bodyH - 2                    // minus the top and bottom border
```

Every number is in terminal cells (characters across, rows down). Lip
Gloss's `Width` and `Height` include padding but not the border or
margin, which is why the border is subtracted by hand.

Lines that could be wider than the terminal (the tab bar and the help
line) are cut off with `MaxWidth(m.width)` instead of wrapping, because a
wrapped line is an extra row.

**The top border.** Lip Gloss can't put text into a border, so
[refresh_view.go](../internal/mvc/refresh_view.go) draws the panel
normally and then replaces its first line (`withButtons`):

```
┌─ [button] ─────────── fill ─────────── updated 14:30:05 ─┐
```

The `─` fill is worked out so the line is exactly as wide as the box. If
the time doesn't fit it is left out; if the button doesn't fit either, the
plain border stays and the cursor skips the button (`buttonsShown`). The
margins are added after `withButtons`, which needs the border to be the
panel's first line.

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
   returns `itemsLoadedMsg{tab: "Pods", …}`.
3. Add `case "Pods": return loadPods(m.client)` to `loadActive` in
   [refresh.go](../internal/mvc/refresh.go).
4. Optionally replace the placeholder rows in `New` with `{"loading..."}`.

`Update` needs no change: it already handles `itemsLoadedMsg` for any
tab, and auto refresh picks the new loader up through `loadActive`.

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
   [refresh_view.go](../internal/mvc/refresh_view.go), and append the
   button in `refreshButtons` (or a new function that returns the list).
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

**Known failure:** `TestViewManyRowsDoNotGrowScreen`. The panel's
`Height` is a minimum, so a list with more rows than fit makes the panel
taller than the screen. The fix is to scroll the list: keep a scroll
offset in the model and draw only the rows that fit.
