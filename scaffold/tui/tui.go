package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kxue43/cli-toolkit/scaffold"
)

type (
	DepItem struct {
		scaffold.VersionSetter
		ti textinput.Model
	}

	PythonModel struct {
		cmd         *scaffold.PythonProjectCmd
		Title       string
		Description string
		Deps        []DepItem
		index       int
		navMode     bool
		working     bool
	}
)

func InitialPythonModel(cmd *scaffold.PythonProjectCmd) PythonModel {
	vss := cmd.VersionSetters

	pm := PythonModel{
		Title:       "Python Project",
		Description: "Scaffold a Python project.",
		Deps:        make([]DepItem, len(vss)),
		navMode:     true,
		index:       0,
		cmd:         cmd,
	}

	var ti textinput.Model

	for i := range vss {
		*vss[i].Indirect = "LATEST"

		ti = textinput.New()
		ti.Placeholder = "LATEST"
		ti.CharLimit = 128
		ti.Width = 20
		ti.Prompt = " "

		pm.Deps[i] = DepItem{
			VersionSetter: vss[i],
			ti:            ti,
		}
	}

	return pm
}

func (pm PythonModel) Init() tea.Cmd {
	return nil
}

func (pm PythonModel) View() string {
	if pm.working {
		return "I'm working on it ..."
	}

	var b strings.Builder

	for i, item := range pm.Deps {
		if i == pm.index {
			b.WriteString("> ")
		} else {
			b.WriteString("  ")
		}

		b.WriteString(item.Name)
		b.WriteString(":")
		b.WriteString(item.ti.View())
		b.WriteString("\n")
	}

	if pm.index == len(pm.Deps) {
		b.WriteString("\n> [ Submit ]\n")
	} else {
		b.WriteString("\n  [ Submit ]\n")
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", pm.Title, pm.Description, b.String())
}

func (pm *PythonModel) navModeUpdate(msg tea.Msg) (cmd tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.Type {
	case tea.KeyUp:
		if pm.index > 0 {
			pm.index -= 1
		}

		return nil
	case tea.KeyDown:
		if pm.index < len(pm.Deps) {
			pm.index += 1
		}

		return nil
	case tea.KeyEnter:
		pm.navMode = false

		cmd = pm.Deps[pm.index].ti.Focus()

		return cmd
	default:
		return nil
	}
}

func (pm *PythonModel) backToNavMode() {
	pm.Deps[pm.index].ti.Blur()

	pm.navMode = true
}

func (pm *PythonModel) scaffoldCmd() tea.Msg {
	for i := range pm.Deps {
		if v := pm.Deps[i].ti.Value(); v != "" {
			*pm.Deps[i].Indirect = v
		}
	}

	pm.cmd.ProjectName = "fs-walk"
	pm.cmd.PythonVersion = scaffold.PythonVersion{Major: "3", Minor: "12"}
	pm.cmd.TimeoutSeconds = 1

	_ = pm.cmd.AfterApply()

	_ = pm.cmd.Run()

	return tea.Quit()
}

func (pm PythonModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok && keyMsg.Type == tea.KeyEsc {
		return pm, tea.Quit
	} else if ok && keyMsg.Type == tea.KeyEnter && pm.index == len(pm.Deps) {
		pm.working = true

		return pm, pm.scaffoldCmd
	}

	if pm.navMode {
		cmd = pm.navModeUpdate(msg)

		return pm, cmd
	}

	if ok && (keyMsg.Type == tea.KeyEnter) {
		pm.backToNavMode()

		return pm, nil
	}

	pm.Deps[pm.index].ti, cmd = pm.Deps[pm.index].ti.Update(msg)

	return pm, cmd
}
