package tui

import (
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

	pythonDeps struct {
		help    help.Model
		cmd     *scaffold.PythonProjectCmd
		deps    []depItem
		index   int
		navMode bool
		working bool
	}

	navModeKeyMap struct{}

	inputModeKeyMap struct{}
)

var (
	keys = struct {
		up      key.Binding
		down    key.Binding
		select_ key.Binding
		help    key.Binding
		quit    key.Binding
	}{
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

func (navModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keys.help, keys.quit}
}

func (navModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keys.up, keys.down, keys.select_},
		{keys.help, keys.quit},
	}
}

func (inputModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keys.select_, keys.quit}
}

func (inputModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keys.select_, keys.quit},
		{keys.select_, keys.quit},
	}
}

func InitialPythonModel(cmd *scaffold.PythonProjectCmd) pythonDeps {
	vss := cmd.VersionSetters

	m := pythonDeps{
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

		m.deps[i] = depItem{
			VersionSetter: vss[i],
			ti:            ti,
		}
	}

	return m
}

func (m pythonDeps) Init() tea.Cmd {
	return nil
}

func (m pythonDeps) View() string {
	if m.working {
		return "I'm working on it ...\n"
	}

	var b strings.Builder

	for i, item := range m.deps {
		if i == m.index {
			b.WriteString("> ")
		} else {
			b.WriteString("  ")
		}

		b.WriteString(item.Name)
		b.WriteRune(':')
		b.WriteString(item.ti.View())
		b.WriteRune('\n')
	}

	if m.index == len(m.deps) {
		b.WriteString("\n> [ Submit ]\n\n")
	} else {
		b.WriteString("\n  [ Submit ]\n\n")
	}

	if m.navMode {
		b.WriteString(indentedStyle.Render(m.help.View(navModeKeyMap{})))
	} else {
		b.WriteString(indentedStyle.Render(m.help.View(inputModeKeyMap{})))
	}

	b.WriteRune('\n')

	return b.String()
}

func (m *pythonDeps) navModeUpdate(msg tea.KeyMsg) (cmd tea.Cmd) {
	switch {
	case key.Matches(msg, keys.up):
		if m.index > 0 {
			m.index -= 1
		}

		return nil
	case key.Matches(msg, keys.down):
		if m.index < len(m.deps) {
			m.index += 1
		}

		return nil
	case key.Matches(msg, keys.select_):
		m.navMode = false
		m.help.ShowAll = false

		cmd = m.deps[m.index].ti.Focus()

		return cmd
	default:
		return nil
	}
}

func (m *pythonDeps) backToNavMode() {
	m.deps[m.index].ti.Blur()

	m.navMode = true
}

func (m *pythonDeps) scaffoldCmd() tea.Msg {
	for i := range m.deps {
		if v := m.deps[i].ti.Value(); v != "" {
			*m.deps[i].Indirect = v
		}
	}

	m.cmd.ProjectName = "fs-walk"
	m.cmd.PythonVersion = scaffold.PythonVersion{Major: "3", Minor: "12"}
	m.cmd.TimeoutSeconds = 1

	_ = m.cmd.AfterApply()

	_ = m.cmd.Run()

	return tea.Quit()
}

func (m pythonDeps) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.help.Width = msg.Width

		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.quit):
			return m, tea.Quit
		case m.index == len(m.deps) && key.Matches(msg, keys.select_):
			m.working = true

			return m, m.scaffoldCmd
		case m.navMode && key.Matches(msg, keys.help):
			m.help.ShowAll = !m.help.ShowAll

			return m, nil
		case m.navMode:
			cmd = m.navModeUpdate(msg)

			return m, cmd
		case !m.navMode && key.Matches(msg, keys.select_):
			m.backToNavMode()

			return m, nil
		case !m.navMode:
			m.deps[m.index].ti, cmd = m.deps[m.index].ti.Update(msg)

			return m, cmd
		default:
		}
	}

	return m, nil
}
