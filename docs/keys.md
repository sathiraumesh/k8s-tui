# Keys

All key bindings are constants in
[internal/mvc/keys.go](../internal/mvc/keys.go). To rebind a key, change
its value there.

## Where the cursor can be

The cursor (the *focus*) is always in one of three places. The arrow keys
do different things in each.

```
  Nodes      Pods      Deployments      Services        ← 1. tabs

┌─  refresh: 5s  ─────────────────── updated 14:30:05 ─┐ ← 2. refresh button
│ controlplane         Ready      v1.37.2              │ ← 3. list
│ worker1              Ready      v1.37.2              │
└──────────────────────────────────────────────────────┘
```

The part the cursor is on is underlined: the active tab, the refresh
button, or the selected row. The app starts with the cursor on the tabs.

```
tabs  ──↓──▶  refresh button  ──↓──▶  list
      ◀──↑──                  ◀──↑──  (from the list's top row)
```

If the terminal is too narrow for the refresh button, it isn't drawn and
the cursor skips it: `↓` from the tabs goes straight to the list.

## On the tabs

| Key | Does |
|---|---|
| `←` / `h` | previous tab |
| `→` / `l` | next tab |
| `↓` / `j` | go to the refresh button |

Switching tabs loads the new tab straight away and puts the list's cursor
back on the first row. The tabs wrap around: `→` on the last tab goes to
the first.

## On the refresh button

| Key | Does |
|---|---|
| `enter` / `space` | step the refresh rate: off → 2s → 5s → 10s → off |
| `←` / `→` | move between buttons (there is only one for now) |
| `↑` / `k` | go to the tabs |
| `↓` / `j` | go to the list |

## On the list

| Key | Does |
|---|---|
| `↓` / `j` | next row (stops at the last one) |
| `↑` / `k` | previous row; on the first row, go to the refresh button |
| `←` / `→` | switch tab |

## Anywhere

| Key | Does |
|---|---|
| `tab` | step the focus: tabs → refresh button → list → tabs |
| `shift+tab` | previous tab |
| `r` | step the refresh rate, the same as pressing the refresh button |
| `q` / `ctrl+c` | quit |

## Auto refresh

The refresh button shows the rate: `refresh: 5s` (green) while it's on,
`refresh: off` (dark gray) while it's off. The app starts at 5 seconds.

While it's on, only the tab you're looking at is reloaded. The right end
of the panel's top border shows when that tab was last loaded, for
example `updated 14:30:05`. A failed reload shows the error in the panel
and keeps the time of the last load that worked.

Turning auto refresh back on from off reloads straight away.
