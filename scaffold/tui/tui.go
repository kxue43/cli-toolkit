package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kxue43/cli-toolkit/scaffold"
)

type (
	depItem struct {
		scaffold.VersionSetter
		ti textinput.Model
	}

	pythonModel struct {
		cmd     *scaffold.PythonProjectCmd
		title   string
		desc    string
		deps    []depItem
		index   int
		navMode bool
		working bool
	}
)

func InitialPythonModel(cmd *scaffold.PythonProjectCmd) pythonModel {
	vss := cmd.VersionSetters

	pm := pythonModel{
		title:   "Python Project",
		desc:    "Scaffold a Python project.",
		deps:    make([]depItem, len(vss)),
		navMode: true,
		index:   0,
		cmd:     cmd,
	}

	var ti textinput.Model

	for i := range vss {
		*vss[i].Indirect = "LATEST"

		ti = textinput.New()
		ti.Placeholder = "LATEST"
		ti.CharLimit = 128
		ti.Width = 20
		ti.Prompt = " "

		pm.deps[i] = depItem{
			VersionSetter: vss[i],
			ti:            ti,
		}
	}

	return pm
}

func (pm pythonModel) Init() tea.Cmd {
	return nil
}

func (pm pythonModel) View() string {
	if pm.working {
		return "I'm working on it ..."
	}

	var b strings.Builder

	for i, item := range pm.deps {
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

	if pm.index == len(pm.deps) {
		b.WriteString("\n> [ Submit ]\n")
	} else {
		b.WriteString("\n  [ Submit ]\n")
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", pm.title, pm.desc, b.String())
}

func (pm *pythonModel) navModeUpdate(msg tea.Msg) (cmd tea.Cmd) {
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
		if pm.index < len(pm.deps) {
			pm.index += 1
		}

		return nil
	case tea.KeyEnter:
		pm.navMode = false

		cmd = pm.deps[pm.index].ti.Focus()

		return cmd
	default:
		return nil
	}
}

func (pm *pythonModel) backToNavMode() {
	pm.deps[pm.index].ti.Blur()

	pm.navMode = true
}

func (pm *pythonModel) scaffoldCmd() tea.Msg {
	for i := range pm.deps {
		if v := pm.deps[i].ti.Value(); v != "" {
			*pm.deps[i].Indirect = v
		}
	}

	pm.cmd.ProjectName = "fs-walk"
	pm.cmd.PythonVersion = scaffold.PythonVersion{Major: "3", Minor: "12"}
	pm.cmd.TimeoutSeconds = 1

	_ = pm.cmd.AfterApply()

	_ = pm.cmd.Run()

	return tea.Quit()
}

func (pm pythonModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok && keyMsg.Type == tea.KeyEsc {
		return pm, tea.Quit
	} else if ok && keyMsg.Type == tea.KeyEnter && pm.index == len(pm.deps) {
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

	pm.deps[pm.index].ti, cmd = pm.deps[pm.index].ti.Update(msg)

	return pm, cmd
}
