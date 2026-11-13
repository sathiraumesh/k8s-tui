package mvc

import (
	"strings"

	"k8s-tui/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

// Drawing the start-up screen (splash.go), in the middle of the window:
//
//	█   █  ███   ████     █████ █   █ ███
//	█  █  █   █ █           █   █   █  █
//	███    ███   ███        █   █   █  █
//	█  █  █   █     █       █   █   █  █
//	█   █  ███  ████        █    ███  ███
//
//	              kubernetes
//
//	     context kubernetes-admin@kubernetes
//
//	         ⠹ loading the cluster
//
// A band of white sweeps across the blue letters, left to right, again and
// again. In a window too small for the big letters the name is on one line.

// bannerText is what the big letters spell; each character needs a glyph.
const bannerText = "K8S TUI"

// bannerGlyphs are the big letters, five rows high, "#" for a filled cell.
// Every row of a glyph is as wide as the others.
var bannerGlyphs = map[rune][]string{
	'K': {"#   #", "#  # ", "###  ", "#  # ", "#   #"},
	'8': {" ### ", "#   #", " ### ", "#   #", " ### "},
	'S': {" ####", "#    ", " ### ", "    #", "#### "},
	'T': {"#####", "  #  ", "  #  ", "  #  ", "  #  "},
	'U': {"#   #", "#   #", "#   #", "#   #", " ### "},
	'I': {"###", " # ", " # ", " # ", "###"},
	' ': {"  ", "  ", "  ", "  ", "  "},
}

// bannerRows is how many rows high the big letters are.
const bannerRows = 5

// sweepWidth is how many columns wide the white band is.
const sweepWidth = 4

// spinner is the turning wheel in front of the loading line, one picture per frame.
var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	logoStyle  = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	shineStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.OnAccent)
)

// banner is bannerText in big letters, one empty column between letters,
// with "#" for a filled cell.
func banner() []string {
	rows := make([]string, bannerRows)
	for i := range rows {
		var glyphs []string
		for _, r := range bannerText {
			glyphs = append(glyphs, bannerGlyphs[r][i])
		}
		rows[i] = strings.Join(glyphs, " ")
	}
	return rows
}

// sweepAt is the first column of the white band in picture frame. The band
// starts left of the letters and moves 2 columns a picture until it is past
// them, then starts again.
func sweepAt(frame, width int) int {
	lap := width + 2*sweepWidth // from fully left of the letters to fully right of them
	return (frame*2)%lap - sweepWidth
}

// shine draws the big letters for picture frame: filled cells blue, and
// white where the band is.
func shine(rows []string, frame int) []string {
	width := len(rows[0])
	from := sweepAt(frame, width)
	out := make([]string, len(rows))
	for i, row := range rows {
		var b strings.Builder
		for col, ch := range row {
			switch {
			case ch == ' ':
				b.WriteByte(' ')
			case col >= from && col < from+sweepWidth:
				b.WriteString(shineStyle.Render("█"))
			default:
				b.WriteString(logoStyle.Render("█"))
			}
		}
		out[i] = b.String()
	}
	return out
}

// splashView is the start-up screen, centred in the window.
func (m Model) splashView() string {
	big := banner()
	var lines []string
	if len(big[0]) <= m.width && bannerRows+6 <= m.height {
		lines = shine(big, m.splashFrame)
	} else {
		lines = []string{logoStyle.Render("k8s-tui")} // too small for the big letters
	}

	lines = append(lines, "", logoStyle.Render("kubernetes"), "")
	if m.client != nil && m.client.Context() != "" {
		lines = append(lines, Help.Render("context ")+commandStyle.Render(m.client.Context()), "")
	}
	frame := spinner[m.splashFrame%len(spinner)]
	lines = append(lines, logoStyle.Render(frame)+Help.Render(" loading the cluster"))

	// Each line centred on its own, then the block in the middle of the window.
	block := lipgloss.JoinVertical(lipgloss.Center, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block)
}
