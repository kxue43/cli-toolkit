package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kxue43/cli-toolkit/scaffold"
)

type (
	depItem struct {
		scaffold.VersionSetter
		ti textinput.Model
	}

	pythonModel struct {
		help    help.Model
		cmd     *scaffold.PythonProjectCmd
		title   string
		desc    string
		deps    []depItem
		index   int
		navMode bool
		working bool
	}

	keyMap struct {
		up      key.Binding
		down    key.Binding
		select_ key.Binding
		help    key.Binding
		quit    key.Binding
	}
)

var (
	keys = keyMap{
		up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		select_: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("\u21B5", "select/de-select"),
		),
		help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		quit: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "quit"),
		),
	}

	indentedStyle = lipgloss.NewStyle().PaddingLeft(2)
)

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.help, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.up, k.down, k.select_},
		{k.help, k.quit},
	}
}

func InitialPythonModel(cmd *scaffold.PythonProjectCmd) pythonModel {
	vss := cmd.VersionSetters

	pm := pythonModel{
		title:   "Python Project",
		desc:    "Scaffold a Python project.",
		deps:    make([]depItem, len(vss)),
		navMode: true,
		index:   0,
		cmd:     cmd,
		help:    help.New(),
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
		return "I'm working on it ...\n"
	}

	var b strings.Builder

	for i, item := range pm.deps {
		if i == pm.index {
			b.WriteString("> ")
		} else {
			b.WriteString("  ")
		}

		b.WriteString(item.Name)
		b.WriteRune(':')
		b.WriteString(item.ti.View())
		b.WriteRune('\n')
	}

	if pm.index == len(pm.deps) {
		b.WriteString("\n> [ Submit ]\n\n")
	} else {
		b.WriteString("\n  [ Submit ]\n\n")
	}

	b.WriteString(indentedStyle.Render(pm.help.View(keys)))

	b.WriteRune('\n')

	return fmt.Sprintf("%s\n\n%s\n\n%s", pm.title, pm.desc, b.String())
}

func (pm *pythonModel) navModeUpdate(msg tea.KeyMsg) (cmd tea.Cmd) {
	switch {
	case key.Matches(msg, keys.up):
		if pm.index > 0 {
			pm.index -= 1
		}

		return nil
	case key.Matches(msg, keys.down):
		if pm.index < len(pm.deps) {
			pm.index += 1
		}

		return nil
	case key.Matches(msg, keys.select_):
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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		pm.help.Width = msg.Width

		return pm, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.quit):
			return pm, tea.Quit
		case pm.index == len(pm.deps) && key.Matches(msg, keys.select_):
			pm.working = true

			return pm, pm.scaffoldCmd
		case pm.navMode && key.Matches(msg, keys.help):
			pm.help.ShowAll = !pm.help.ShowAll

			return pm, nil
		case pm.navMode:
			cmd = pm.navModeUpdate(msg)

			return pm, cmd
		case !pm.navMode && key.Matches(msg, keys.select_):
			pm.backToNavMode()

			return pm, nil
		case !pm.navMode:
			pm.deps[pm.index].ti, cmd = pm.deps[pm.index].ti.Update(msg)

			return pm, cmd
		default:
		}
	}

	return pm, nil
}
