# Keys

All key bindings are constants in
[internal/mvc/keys.go](../internal/mvc/keys.go). To rebind a key, change
its value there.

## Where the cursor can be

The cursor (the *focus*) is always in one of three places. The arrow keys
do different things in each.

```
  Nodes      Pods      Deployments      Services                 ← 1. tabs

┌─  wide: off  ───  refresh: 5s  ──────────── updated 14:30:05 ─┐ ← 2. buttons
│ controlplane   Ready   v1.37.2                                │ ← 3. list
│ worker1        Ready   v1.37.2                                │
└───────────────────────────────────────────────────────────────┘
```

The part the cursor is on is underlined: the active tab, a button, or the
selected row. The app starts with the cursor on the tabs.

```
tabs  ──↓──▶  buttons  ──↓──▶  list
      ◀──↑──           ◀──↑──  (from the list's top row)
```

If the terminal is too narrow for the buttons, they aren't drawn and the
cursor skips them: `↓` from the tabs goes straight to the list.

## On the tabs

| Key | Does |
|---|---|
| `←` / `h` | previous tab |
| `→` / `l` | next tab |
| `↓` / `j` | go to the buttons |

Switching tabs loads the new tab straight away and puts the list's cursor
back on the first row. The tabs wrap around: `→` on the last tab goes to
the first.

## On the buttons

Coming down from the tabs, the cursor lands on the first button, `wide`.

| Key | Does |
|---|---|
| `←` / `→` | move between the buttons (stops at the first and the last) |
| `enter` / `space` | press the button: switch wide on/off, or step the refresh rate |
| `↑` / `k` | go to the tabs |
| `↓` / `j` | go to the list |

## On the list

| Key | Does |
|---|---|
| `↓` / `j` | next row (stops at the last one) |
| `↑` / `k` | previous row; on the first row, go to the buttons |
| `pgdown` / `pgup` | a page (the rows that fit in the panel) down or up |
| `←` / `→` | switch tab |

The bottom border shows the kubectl command that lists what you see (with
`-o wide` while wide is on). When the list is longer than the panel, it
scrolls to keep the cursor's row on screen, and the right end of the
bottom border shows which rows you see:

```
└─ $ kubectl get nodes ─────────────────────────── 4-12 of 30 ─┘
```

Rows wider than the panel are cut off at its edge. Switching tabs scrolls
back to the top.

## Actions on a row

`enter` on a row opens what can be done to it; a key (or `enter` on one)
picks it. Pods and Deployments share keys where the action is the same:

| Key | Pods | Deployments |
|---|---|---|
| `d` | Describe | Describe |
| `o` | Logs (asks which container if there are several) | |
| `s` | Show labels and annotations | Show labels and annotations |
| `x` | Shell (asks which running container) | |
| `e` | Edit the YAML | Edit the YAML |
| `i` | Set image: pick the container (or all), type the image | Set image (same); rolls out new pods |
| `c` | | Scale: type the replicas |
| `t` | | Restart, like `kubectl rollout restart` |
| `l` | Label | Label |
| `r` | Delete | Delete |

`+` (or the `+ run` / `+ create` button) makes a new pod or deployment
from an image. Anything that changes the cluster asks first (`y` runs it)
and has a `--dry-run=server` tick box.

## Status colors

The status column of a row is colored by how things are:

| Tab | Column | Green | Yellow | Red |
|---|---|---|---|---|
| Nodes | STATUS | `Ready` | `Ready,SchedulingDisabled` (cordoned), `Unknown` | `NotReady` |
| Pods | STATUS | `Running`, `Completed` | `Pending`, `ContainerCreating`, `Init:1/2`, `Terminating` | `CrashLoopBackOff`, `ImagePullBackOff`, `Error`, `OOMKilled`, `Evicted`, … |
| Deployments | READY | all ready (`3/3`) | some ready (`1/3`) | none ready (`0/3`) |

A deployment scaled to `0/0` isn't colored.

The pages of text are colored the same way:

| Page | Red | Yellow | Green |
|---|---|---|---|
| Describe | `CrashLoopBackOff`, `Error`, `OOMKilled`, `Failed…`, `BackOff`, `Unhealthy`, `NotReady`, … | `Warning` events (the whole line), `Waiting`, `Pending`, `Terminated`, `Unknown`, … | `Running`, `Completed`, `NewReplicaSetAvailable`, … |
| Logs | lines with `error`, `fatal`, `panic`, `exception`, `failed`, klog `E…` | lines with `warn`/`warning`, klog `W…` | |
| A dry run / change | lines removed (`-`) | | lines added (`+`) |

The `==> pod/container <==` headings of logs are in the accent color.
The line under the panel is red when an action failed and green when it
worked; "This changes the cluster." on a confirm page is yellow, and a
tab's `error: …` row red. Show (labels and annotations) isn't colored.

## Mouse

| Mouse | Does |
|---|---|
| wheel | scrolls one row a step: the list, a page of text, or a page's list. It stays in the panel: at the top it doesn't go up to the buttons |
| click a tab | switches to it (and closes a page that is open) |
| click a button | presses it: `ns`, `wide`, `refresh`, `+ run` / `+ create`, `/ search` |
| click a row | selects it; click it again to open its actions |
| click an action, namespace or container | picks it |
| click the text field | puts the cursor there to type |
| click a tick box | ticks or unticks it |
| click the last line (`y run` / `enter run`) | runs the action |
| right click | back, like `esc` |

While the app reads the mouse, select text with `shift`+drag (most
terminals).

## Anywhere

| Key | Does |
|---|---|
| `tab` | step the focus: tabs → buttons → list → tabs |
| `shift+tab` | previous tab |
| `w` | wide columns on/off, the same as pressing the wide button |
| `r` | step the refresh rate, the same as pressing the refresh button |
| `q` / `ctrl+c` | quit |

## Wide columns

The wide button shows `wide: off` (dark gray) or `wide: on` (green). Wide
adds the columns `kubectl get nodes -o wide` shows:

| Normal | Wide adds |
|---|---|
| NAME, STATUS, VERSION | INTERNAL-IP, EXTERNAL-IP, OS-IMAGE, KERNEL-VERSION, CONTAINER-RUNTIME |
| Pods: NAME, READY, STATUS, RESTARTS, AGE | IP, NODE, NOMINATED-NODE, READINESS-GATES |
| Deployments: NAME, READY, UP-TO-DATE, AVAILABLE, AGE | CONTAINERS, IMAGES, SELECTOR |

An empty external IP shows as `<none>`, as in kubectl.

Each tab has its own setting: switching wide on for Nodes leaves the
other tabs as they were. Switching it reloads the tab with the new
columns; the rows on screen stay until the new ones arrive. Auto refresh
keeps the tab's setting. (Services has no wide columns yet.)

## Auto refresh

The refresh button shows the rate: `refresh: 5s` (green) while it's on,
`refresh: off` (dark gray) while it's off. The app starts at 5 seconds.

While it's on, only the tab you're looking at is reloaded. The right end
of the panel's top border shows when that tab was last loaded, for
example `updated 14:30:05`. A failed reload shows the error in the panel
and keeps the time of the last load that worked.

Turning auto refresh back on from off reloads straight away.
