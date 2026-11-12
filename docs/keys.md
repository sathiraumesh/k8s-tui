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

When the list is longer than the panel, it scrolls to keep the cursor's
row on screen, and the bottom border shows which rows you see:

```
└─ 4-12 of 30 ──────────────────────────────────────────────┘
```

Rows wider than the panel are cut off at its edge. Switching tabs scrolls
back to the top.

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

An empty external IP shows as `<none>`, as in kubectl.

Each tab has its own setting: switching wide on for Nodes leaves the
other tabs as they were. Switching it reloads the tab with the new
columns; the rows on screen stay until the new ones arrive. Auto refresh
keeps the tab's setting. (Pods, Deployments and Services have no wide
columns yet.)

## Auto refresh

The refresh button shows the rate: `refresh: 5s` (green) while it's on,
`refresh: off` (dark gray) while it's off. The app starts at 5 seconds.

While it's on, only the tab you're looking at is reloaded. The right end
of the panel's top border shows when that tab was last loaded, for
example `updated 14:30:05`. A failed reload shows the error in the panel
and keeps the time of the last load that worked.

Turning auto refresh back on from off reloads straight away.
