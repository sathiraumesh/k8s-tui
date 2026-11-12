# k8s-tui

A terminal UI for looking at a Kubernetes cluster, written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss). It talks to the
cluster of your current kubeconfig context, the same one `kubectl` uses.

```
  Nodes      Pods      Deployments      Services

┌─  wide: off  ───  refresh: 5s  ──────────────── updated 14:30:05 ─┐
│ controlplane                   Ready      v1.37.2                  │
│ worker1                        Ready      v1.37.2                  │
│ worker2                        Ready      v1.37.2                  │
└────────────────────────────────────────────────────────────────────┘

tab focus • ←→ tab/button • ↑↓ move • enter press • w wide • r refresh • q quit
```

- **Nodes** are loaded live from the cluster: name, status and kubelet version.
- **Wide** adds the columns of `kubectl get nodes -o wide`: internal and
  external IP, OS image, kernel version and container runtime.
- **Auto refresh** reloads the tab you're on every 2, 5 or 10 seconds, or
  not at all. The time of the last update shows in the top-right corner.
- **Pods, Deployments and Services** show placeholder rows for now; they
  are not loaded from the cluster yet.

## Run

You need Go 1.26 or newer and a working kubeconfig (`~/.kube/config`, or
the file in `$KUBECONFIG`). If `kubectl get nodes` works, this will too.

```sh
go run .
```

or build a binary:

```sh
go build -o k8s-tui .
./k8s-tui
```

If the kubeconfig can't be read, the app prints why and exits. If the
cluster can't be reached, the panel shows the error instead of the rows.

## Keys

| Key | Does |
|---|---|
| `←` `→` (or `h` `l`) | switch tab; on the buttons, move between them |
| `↑` `↓` (or `k` `j`) | move between the tabs, the buttons and the list, and through the list |
| `tab` | step the focus: tabs → buttons → list |
| `shift+tab` | previous tab |
| `enter` / `space` | press the button the cursor is on |
| `w` | wide columns on/off for the tab you're on, from anywhere |
| `r` | change the refresh rate from anywhere: off → 2s → 5s → 10s |
| `q` / `ctrl+c` | quit |

See [docs/keys.md](docs/keys.md) for the details.

## Test

```sh
go test ./...
```

The tests use client-go's fake cluster, so they need no real cluster.
One test, `TestViewManyRowsDoNotGrowScreen`, fails on purpose for now: the
list doesn't scroll yet, so a list longer than the screen pushes the tabs
off the top.

## More

- [Keys](docs/keys.md): every key, and where the cursor can be
- [Architecture](docs/architecture.md): how the code is organised, and how
  to add a tab, a key or a button
