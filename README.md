# k8s-tui

A terminal UI for Kubernetes, written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and client-go.
It uses your current kubeconfig context, the same one `kubectl` uses.

```
  Nodes      Pods      Deployments      Services      More

┌─  wide: off  ───  refresh: 5s  ──────────────── updated 14:30:05 ─┐
│ controlplane                   Ready      v1.37.2                  │
│ worker1                        Ready      v1.37.2                  │
│ worker2                        Ready      v1.37.2                  │
└─ $ kubectl get nodes ──────────────────────────────────────────────┘
```

## Features

- **Nodes**, live from the cluster, with `w` for the wide columns
- **Node actions** (`enter` on a node): describe, show labels and taints,
  shell (`kubectl debug`), cordon, uncordon, label and taint. Flags are tick
  boxes, changes ask for `y`, and `--dry-run=server` shows what would change.
- **More**: every resource type, like `kubectl api-resources`
- Auto refresh (off, 2s, 5s or 10s), scrolling, and the kubectl command for
  what you see on the bottom border

## Run

You need Go 1.26+ and a kubeconfig that works with `kubectl get nodes`.
The Shell action also needs `kubectl` installed.

```sh
go run .
```

## Keys

`←→` tabs • `↑↓` move • `tab` focus • `enter` actions • `w` wide •
`r` refresh rate • `q` quit. More in [docs/keys.md](docs/keys.md).

## Test

```sh
go test ./...
```

The tests use client-go's fake cluster, so they don't need a real one.
How the code is laid out is in [docs/architecture.md](docs/architecture.md).
