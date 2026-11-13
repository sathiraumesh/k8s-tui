package mvc

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	"github.com/charmbracelet/lipgloss"

	"k8s.io/client-go/kubernetes/fake"
)

// ticks sends n splash ticks.
func ticks(t *testing.T, m Model, n int) Model {
	t.Helper()
	for range n {
		m = press(t, m, splashTickMsg{})
	}
	return m
}

var nodesLoaded = itemsLoadedMsg{tab: tabNodes, rows: []string{"worker1   Ready   v1.37.2"}, at: time.Now()}

func TestNewHasNoSplash(t *testing.T) {
	if New(nil).splash {
		t.Error("New shows the splash; only WithSplash should")
	}
}

func TestSplashShownInPlaceOfTabs(t *testing.T) {
	out := sized(t, New(nil).WithSplash(), 80, 24).View()
	if !strings.Contains(out, "█") || !strings.Contains(out, "kubernetes") || !strings.Contains(out, "loading the cluster") {
		t.Errorf("splash screen:\n%s", out)
	}
	if strings.Contains(out, tabNodes) {
		t.Error("the tabs are showing behind the splash")
	}
}

// The splash stays until the nodes have loaded and the fewest pictures were shown.
func TestSplashClosesAfterLoadAndMinFrames(t *testing.T) {
	m := sized(t, New(nil).WithSplash(), 80, 24)
	if m = ticks(t, m, minSplashFrames+5); !m.splash {
		t.Fatal("splash closed before anything loaded")
	}

	m = press(t, sized(t, New(nil).WithSplash(), 80, 24), nodesLoaded)
	if m = ticks(t, m, minSplashFrames-1); !m.splash {
		t.Fatal("splash closed before minSplashFrames pictures")
	}
	if m = ticks(t, m, 1); m.splash {
		t.Fatal("splash still showing after the nodes loaded")
	}
	if !strings.Contains(m.View(), tabNodes) {
		t.Error("no tabs after the splash")
	}
	if m.splashTick() != nil {
		t.Error("the animation timer keeps running after the splash closed")
	}
}

// A cluster that fails to answer closes the splash too, to show the error.
func TestSplashClosesOnError(t *testing.T) {
	m := press(t, sized(t, New(nil).WithSplash(), 80, 24), itemsLoadedMsg{tab: tabNodes, err: errTest})
	if m = ticks(t, m, minSplashFrames); m.splash {
		t.Error("splash still showing after the load failed")
	}
}

// During the splash only quitting works.
func TestSplashKeys(t *testing.T) {
	m := sized(t, New(nil).WithSplash(), 80, 24)
	if m = press(t, m, tabKey, down, key("w")); m.focus != focusTabs || m.activeWide() {
		t.Error("a key did something behind the splash")
	}
	if _, cmd := m.Update(key("q")); cmd == nil {
		t.Error("q didn't quit during the splash")
	}
}

func TestSplashShowsContext(t *testing.T) {
	// A fake client has no context: the line is left out.
	m := sized(t, New(k8s.NewClientFromClientset(fake.NewClientset())).WithSplash(), 80, 24)
	if strings.Contains(m.View(), "context") {
		t.Error("context line shown without a context")
	}
}

// A window too small for the big letters gets the name on one line.
func TestSplashSmallWindow(t *testing.T) {
	out := sized(t, New(nil).WithSplash(), 30, 8).View()
	if strings.Contains(out, "█") || !strings.Contains(out, "k8s-tui") {
		t.Errorf("small splash:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 30 {
			t.Errorf("line %d wide in a 30 wide window: %q", w, line)
		}
	}
}

// The white band starts left of the letters and moves right, then starts again.
func TestSweepMoves(t *testing.T) {
	if sweepAt(0, 36) != -sweepWidth || sweepAt(1, 36) != 2-sweepWidth {
		t.Errorf("sweepAt(0) = %d, sweepAt(1) = %d", sweepAt(0, 36), sweepAt(1, 36))
	}
	lap := (36 + 2*sweepWidth) / 2
	if sweepAt(lap, 36) != sweepAt(0, 36) {
		t.Error("the band doesn't start again after a lap")
	}
}

func TestBannerRowsSameWidth(t *testing.T) {
	rows := banner()
	for _, r := range rows {
		if len(r) != len(rows[0]) {
			t.Errorf("banner rows differ in width: %q", rows)
		}
	}
}
