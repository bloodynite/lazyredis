package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func pasteAt(base, overlay string, col int) string {
	if col <= 0 {
		return overlay + base
	}
	left := ansi.Truncate(base, col, "")
	overlayW := ansi.StringWidth(overlay)
	restStart := col + overlayW
	baseW := ansi.StringWidth(base)
	var right string
	if restStart < baseW {
		right = ansi.Cut(base, restStart, baseW)
	}
	return left + overlay + right
}

func padLine(line string, width int) string {
	gap := width - ansi.StringWidth(line)
	if gap <= 0 {
		return line
	}
	return line + strings.Repeat(" ", gap)
}

func overlayCenter(base, dialog string, width, height int) string {
	dialogW := lipgloss.Width(dialog)
	dialogH := lipgloss.Height(dialog)
	startX := max(0, (width-dialogW)/2)
	var startY int
	if dialogH >= height {
		startY = height - dialogH
	} else {
		startY = (height - dialogH) / 2
	}

	lines := strings.Split(base, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := 0; i < height; i++ {
		lines[i] = padLine(lines[i], width)
	}

	dialogLines := strings.Split(dialog, "\n")
	for i, dl := range dialogLines {
		row := startY + i
		if row < 0 || row >= height {
			continue
		}
		lines[row] = pasteAt(lines[row], dl, startX)
	}
	return strings.Join(lines, "\n")
}

func dimContent(content string) string {
	return lipgloss.NewStyle().Faint(true).Render(content)
}
