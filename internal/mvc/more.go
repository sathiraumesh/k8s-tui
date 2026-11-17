package mvc

import (
	"k8s-tui/internal/mvc/commands"

	tea "github.com/charmbracelet/bubbletea"
)

// Views inside More. More lists every kind of resource; enter on one with a
// tab of its own goes to that tab (Pods), and on one of the kinds below it
// opens a list of that kind in More itself, which then works like any tab:
// its rows, actions, buttons and auto refresh.
//
//	More ──enter on persistentvolumes──▶ More › pv  (the volumes)
//	     ◀──────── esc, right click, or a click on More ─────┘
//
// While one is open, activeTab is that view, so everything that works from
// the active tab (loading, actions, the status colors) works from it.

// kindTabs are the views More can open, each a tab of its own that isn't in
// the tab bar. title is how the tab bar shows it after "More › ".
func kindTabs() []tab {
	return []tab{
		{
			name:        commands.TabPersistentVolumes,
			title:       "pv",
			load:        commands.LoadPersistentVolumes,
			command:     "kubectl get persistentvolumes",
			actions:     commands.PersistentVolumeActions(),
			status:      &commands.PersistentVolumeStatus,
			own:         commands.CreatePersistentVolumeActions(),
			ownButton:   "+ create",
			ownTitle:    "Create a persistent volume",
			autoRefresh: true,
		},
		{
			name:          commands.TabPersistentVolumeClaims,
			title:         "pvc",
			load:          commands.LoadPersistentVolumeClaims,
			command:       "kubectl get persistentvolumeclaims",
			namespaced:    true,
			actions:       commands.PersistentVolumeClaimActions(),
			status:        &commands.PersistentVolumeClaimStatus,
			rowTarget:     commands.NamespacedTarget,
			commandTarget: commands.NamespacedArgs,
			own:           commands.CreatePersistentVolumeClaimActions(),
			ownButton:     "+ create",
			ownTitle:      "Create a claim",
			ownTarget:     commands.RunNamespace,
			autoRefresh:   true,
		},
	}
}

// kindTab is the view of More called name.
func (m Model) kindTab(name string) (tab, bool) {
	for _, t := range m.kinds {
		if t.name == name {
			return t, true
		}
	}
	return tab{}, false
}

// inKind reports whether a view inside More is on screen.
func (m Model) inKind() bool {
	return m.tabs[m.active].name == tabMore && m.moreKind != ""
}

// openKind opens the view called name in More, on its list, and loads it.
// The row of More it was opened from is kept, to go back to.
func (m Model) openKind(name string) (Model, tea.Cmd) {
	t, ok := m.kindTab(name)
	if !ok {
		return m, nil
	}
	m.moreCursor, m.moreOffset = m.cursor, m.offset
	m.moreKind = t.name
	if _, loaded := m.updated[t.name]; !loaded {
		m.items[t.name] = []string{"loading..."}
	}
	m.err = nil
	m.cursor, m.offset = 0, 0
	m.focus = focusList
	return m, m.loadActive()
}

// closeKind goes back from a view to More's list of kinds, on the row it
// was opened from.
func (m Model) closeKind() Model {
	m.moreKind = ""
	m.err = nil
	m.cursor, m.offset = m.moreCursor, m.moreOffset
	m.focus = focusList
	return m.keepCursorVisible()
}

// tabLabel is what the tab bar shows for tab i: its name, and for More
// while a view is open, the view too: "More › pv".
func (m Model) tabLabel(i int) string {
	if i == m.active && m.inKind() {
		if t, ok := m.kindTab(m.moreKind); ok {
			return m.tabs[i].name + " › " + t.title
		}
	}
	return m.tabs[i].name
}
