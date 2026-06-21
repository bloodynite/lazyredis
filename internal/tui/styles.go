package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

var (
	subtitleStyle     lipgloss.Style
	statusStyle       lipgloss.Style
	errorStyle        lipgloss.Style
	helpStyle         lipgloss.Style
	keybarStyle       lipgloss.Style
	keyLabelStyle     lipgloss.Style
	keyDescStyle      lipgloss.Style
	keySepStyle       lipgloss.Style
	statusBarStyle    lipgloss.Style
	headerBarStyle    lipgloss.Style
	selectedStyle         lipgloss.Style
	searchMatchStyle      lipgloss.Style
	activeSearchMatchStyle lipgloss.Style
	normalStyle           lipgloss.Style
	typeStringStyle   lipgloss.Style
	typeHashStyle     lipgloss.Style
	typeListStyle     lipgloss.Style
	typeSetStyle      lipgloss.Style
	typeZSetStyle     lipgloss.Style
	panelStyle        lipgloss.Style
	panelFocusedStyle lipgloss.Style
	panelTitleStyle   lipgloss.Style
	infoBarStyle      lipgloss.Style
	confirmModalStyle   lipgloss.Style
	confirmMsgStyle     lipgloss.Style
	confirmHintStyle    lipgloss.Style
	helpGroupTitleStyle lipgloss.Style
)

func initStyles() {
	muted := lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
	faint := lipgloss.AdaptiveColor{Light: "241", Dark: "238"}

	subtitleStyle = lipgloss.NewStyle().Foreground(faint)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	helpStyle = lipgloss.NewStyle().Foreground(muted)

	keybarStyle = lipgloss.NewStyle().Padding(0, 0)
	keyLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	keyDescStyle = lipgloss.NewStyle()
	keySepStyle = lipgloss.NewStyle().Foreground(faint)

	statusBarStyle = lipgloss.NewStyle().Padding(0, 0)
	headerBarStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4")).Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	searchMatchStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("3")).
			Foreground(lipgloss.Color("0")).
			Bold(true)
	// activeSearchMatchStyle marks the match the user is currently navigated
	// to (via n/N). It must stay visually distinct from searchMatchStyle so
	// the active hit stands out from other matches in the same chunk.
	activeSearchMatchStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("1")).
				Foreground(lipgloss.Color("7")).
				Bold(true)
	normalStyle = lipgloss.NewStyle()

	typeStringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	typeHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	typeListStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	typeSetStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	typeZSetStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(faint).
		Padding(0, 1)
	panelFocusedStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(0, 1)
	panelTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(muted)

	infoBarStyle = lipgloss.NewStyle().Padding(0, 1)

	confirmModalStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(1, 2)
	confirmMsgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	confirmHintStyle = lipgloss.NewStyle().Foreground(faint)
	// helpGroupTitleStyle separates each scope group (Global, Browser · Keys
	// panel, …) inside the help modal so the user can scan the reference by
	// context.
	helpGroupTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
}

func configureNewKeyTextarea(t *textarea.Model) {
	lineNum := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "238", Dark: "240"}).
		Faint(true)
	activeLineNum := lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")).
		Bold(true)
	text := lipgloss.NewStyle()
	placeholder := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "241", Dark: "238"}).
		Faint(true)
	prompt := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).
		Faint(true)
	cursorLine := lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"})

	focused := textarea.Style{
		Base:             lipgloss.NewStyle(),
		CursorLine:       cursorLine,
		CursorLineNumber: activeLineNum,
		EndOfBuffer:      lipgloss.NewStyle(),
		LineNumber:       lineNum,
		Placeholder:      placeholder,
		Prompt:           prompt,
		Text:             text,
	}
	blurred := focused
	blurred.CursorLine = lipgloss.NewStyle()

	t.ShowLineNumbers = true
	t.Prompt = "│ "
	t.FocusedStyle = focused
	t.BlurredStyle = blurred
}

func init() {
	initStyles()
}

func typeStyle(t string) lipgloss.Style {
	switch t {
	case "string":
		return typeStringStyle
	case "hash":
		return typeHashStyle
	case "list":
		return typeListStyle
	case "set":
		return typeSetStyle
	case "zset":
		return typeZSetStyle
	default:
		return normalStyle
	}
}
