package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bloodynite/lazyredis/internal/tui"
	"github.com/bloodynite/lazyredis/internal/version"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("Lazyredis %s\n", version.String())
		return
	}
	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
