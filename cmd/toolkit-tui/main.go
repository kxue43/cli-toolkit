package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kxue43/cli-toolkit/scaffold"
	"github.com/kxue43/cli-toolkit/tui"
)

func main() {
	cmd := &scaffold.PythonProjectCmd{}

	_ = cmd.BeforeReset()

	p := tea.NewProgram(tui.InitialPythonModel(cmd))
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
