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
picks it. Pods, Deployments and Services share keys where the action is
the same:

| Key | Pods | Deployments | Services |
|---|---|---|---|
| `d` | Describe | Describe | Describe |
| `o` | Logs (asks which container if there are several) | | |
| `s` | Show labels and annotations | Show labels and annotations | Show labels, annotations and selector |
| `x` | Shell (asks which running container) | | |
| `e` | Edit the YAML | Edit the YAML | Edit the YAML |
| `i` | Set image: pick the container (or all), type the image | Set image (same); rolls out new pods | |
| `c` | | Scale: type the replicas | |
| `t` | | Restart, like `kubectl rollout restart` | |
| `p` | | Expose: make a service for it, e.g. `80:8080` | Endpoints: the pods it sends traffic to, ready or not |
| `f` | | | Port-forward: e.g. `8080:80` makes `localhost:8080` its port 80; `ctrl+c` stops it |
| `l` | Label | Label | Label |
| `r` | Delete | Delete | Delete (its pods keep running) |

`+` (or the `+ run` / `+ create` button) makes something new: a pod or a
deployment from an image, or a service (ClusterIP, NodePort or
LoadBalancer) from a name and ports, e.g. `web 80:8080`, for the pods
labelled `app=web`. Anything that changes the cluster asks first (`y`
runs it) and has a `--dry-run=server` tick box.

### On More

More lists every kind of resource the cluster serves. `enter` (or a
second click) on one that has a tab of its own goes to that tab:
`pods` → Pods, `deployments` → Deployments, `services` → Services,
`nodes` → Nodes, with the cursor on its list. A kind matches by its API
version and kind, so `pods` of `metrics.k8s.io` doesn't go to Pods.

Some kinds without a tab open inside More, which then works like a tab of
its own (rows, actions, buttons, auto refresh); the tab bar shows
`More › pv`. `esc`, a right click or a click on More goes back to the
list of kinds, on the row you came from. Other kinds say they have no
view yet.

**`persistentvolumes`** (`More › pv`): NAME, CAPACITY, ACCESS-MODES,
RECLAIM-POLICY, STATUS (Bound and Available green, Released yellow,
Failed red), CLAIM, STORAGECLASS, REASON, AGE; wide adds VOLUMEMODE.

| Key | Does |
|---|---|
| `d` | Describe: its claim, where its storage is, events |
| `s` | Show labels and annotations |
| `e` | Edit the YAML |
| `p` | Reclaim policy: `Retain` keeps it and its data when its claim is deleted, `Delete` deletes it |
| `a` | Make available: frees a Released volume for a new claim, keeping its data (refused while it is Bound) |
| `l` | Label (a claim can pick a volume by its labels) |
| `r` | Delete (while a claim is bound to it, it waits as Terminating) |

`+ create` makes a hostPath volume, a directory on the node, from a name,
size and path (`data 1Gi /mnt/data`): storage for trying things out on a
one-node or kind cluster.

**`persistentvolumeclaims`** (`More › pvc`): NAMESPACE, NAME, STATUS
(Bound green, Pending yellow, Lost red), VOLUME, CAPACITY, ACCESS-MODES,
STORAGECLASS, AGE; wide adds VOLUMEMODE. Claims are in a namespace, so the
view has the `ns` button.

| Key | Does |
|---|---|
| `d` | Describe: its volume, the pods using it, events |
| `s` | Show labels and annotations |
| `u` | Used by: the pods that mount it (while one does, it can't be deleted for good) |
| `c` | Resize: type a bigger size, e.g. `2Gi` (only if its storage class allows expansion) |
| `e` | Edit the YAML |
| `l` | Label |
| `r` | Delete (its volume then goes by its reclaim policy) |

`+ create` makes a claim from a name, a size and maybe a storage class:
`data 1Gi` (the default class), `data 1Gi standard`, or `data 1Gi -` for
no class, to bind to a hostPath volume made in `More › pv`.

### Other kinds in More

The kinds below open in More with the columns `kubectl get` prints (the
API server's own; `w` for its `-o wide` ones), the `ns` button if they are
in a namespace, and these actions:

| Key | Does |
|---|---|
| `d` | Describe (kubectl's describer) |
| `y` | YAML: the whole object, as `kubectl get -o yaml` prints it |
| `s` | Show labels and annotations |
| `e` | Edit the YAML |
| `l` | Label |
| `r` | Delete |

plus their own:

| Kind | View | Its own |
|---|---|---|
| configmaps | `More › cm` | `a` Data: each key and value; `+ create` from entries: `app-config mode=fast level=3` |
| secrets | `More › secret` | `a` Decode: each value decoded from base64; `+ create` generic from entries (`db-login user=admin password=s3cret`) or docker-registry (`regcred ghcr.io me PASSWORD`) |
| namespaces | `More › ns` | STATUS colored (Active green, Terminating yellow); `u` Use: makes it the namespace the tabs show; Delete warns it deletes everything in it; `+ create` a namespace |
| serviceaccounts | `More › sa` | `t` Token: a short-lived token to act as it (`1h`), like `kubectl create token`; right click › Copy row copies it whole; `+ create` a service account |
| statefulsets | `More › sts` | READY colored; `c` Scale, `t` Restart (one pod at a time, the highest first), `i` Set image; Delete keeps their storage |
| daemonsets | `More › ds` | `t` Restart (node by node), `i` Set image; no Scale: there is one pod per node |
| replicasets | `More › rs` | `c` Scale (a deployment that owns it sets it back) |
| jobs | `More › job` | STATUS colored (Complete green, Running/Suspended yellow, Failed red); `o` Logs of its pods; `p` Suspend, `u` Resume; `+ create`: `hello busybox echo hi` |
| cronjobs | `More › cj` | `n` Run now (a job from it, like `kubectl create job --from=cronjob/NAME`); `p` Suspend, `u` Resume; `+ create`: `backup busybox */5 * * * * echo hi` or `@hourly` |
| ingresses | `More › ing` | `+ create` from rules `HOST/PATH=SERVICE:PORT` (`shop shop.example.com/*=web:80`; a path ending in `*` is a prefix) |
| storageclasses | `More › sc` | `m` Make default (the old default stops being it) |
| ingressclasses | `More › ingressclass` | `m` Make default |
| networkpolicies | `More › netpol` | `+ create`: deny all ingress, or allow only the same namespace, for every pod of the namespace (needs a plugin that enforces policies) |
| horizontalpodautoscalers | `More › hpa` | `+ create` for a deployment: `web 1 5 80` (min, max, CPU %), like `kubectl autoscale`; needs metrics-server |
| poddisruptionbudgets | `More › pdb` | `+ create`: `web app=web 1` (name, selector, min available: a number or `50%`) |
| priorityclasses | `More › pc` | `+ create`: `high 1000` (name, value) |


The status column of a row is colored by how things are:

| Tab | Column | Green | Yellow | Red |
|---|---|---|---|---|
| Nodes | STATUS | `Ready` | `Ready,SchedulingDisabled` (cordoned), `Unknown` | `NotReady` |
| Pods | STATUS | `Running`, `Completed` | `Pending`, `ContainerCreating`, `Init:1/2`, `Terminating` | `CrashLoopBackOff`, `ImagePullBackOff`, `Error`, `OOMKilled`, `Evicted`, … |
| Deployments | READY | all ready (`3/3`) | some ready (`1/3`) | none ready (`0/3`) |
| Services | EXTERNAL-IP | | `<pending>` (a load balancer still waiting for its address) | |

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
| drag | selects text, as in the terminal, and copies it when you let go; it stays shown until the next key or click |
| right click | a menu where you clicked: **Copy** (the selection), **Copy row**, **Copy all** (every row, also those off screen), **Copy command** (the kubectl command), **Back** (on a page) |

Copying uses OSC 52: the app asks the terminal to put the text on the
clipboard, so it works without a clipboard program and over ssh. A
terminal without OSC 52 ignores it. To paste into a text field, use the
terminal's paste (`ctrl+shift+v`).

`K8S_TUI_MOUSE=0` leaves the mouse to the terminal instead: its own
selection and right-click menu, and the wheel as ↑/↓.

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
| Services: NAME, TYPE, CLUSTER-IP, EXTERNAL-IP, PORT(S), AGE | SELECTOR |

An empty external IP shows as `<none>`, as in kubectl.

Each tab has its own setting: switching wide on for Nodes leaves the
other tabs as they were. Switching it reloads the tab with the new
columns; the rows on screen stay until the new ones arrive. Auto refresh
keeps the tab's setting.

## Auto refresh

The refresh button shows the rate: `refresh: 5s` (green) while it's on,
`refresh: off` (dark gray) while it's off. The app starts at 5 seconds.

While it's on, only the tab you're looking at is reloaded. The right end
of the panel's top border shows when that tab was last loaded, for
example `updated 14:30:05`. A failed reload shows the error in the panel
and keeps the time of the last load that worked.

Turning auto refresh back on from off reloads straight away.
