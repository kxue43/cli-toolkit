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
	depsGroup struct {
		name        string
		desc        string
		members     []*depItem
		highlighted bool
		drop        bool
	}

	depItem struct {
		group *depsGroup
		desc  string
		scaffold.VersionSetter
		ti          textinput.Model
		highlighted bool
		drop        bool
	}

	navItem interface {
		ToggleHighlight()
		ToggleTick()
		Desc() string
		Update(tea.Msg) tea.Cmd
		View() string
	}

	pythonDeps struct {
		help    help.Model
		cmd     *scaffold.PythonProjectCmd
		items   []navItem
		index   int
		navMode bool
		working bool
	}

	navModeKeyMap struct{}

	inputModeKeyMap struct{}

	submitButtonKeyMap struct{}
)

var (
	keys = struct {
		up     key.Binding
		down   key.Binding
		input  key.Binding
		finish key.Binding
		submit key.Binding
		tick   key.Binding
		help   key.Binding
		quit   key.Binding
	}{
		up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		input: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("\u21B5", "input mode"),
		),
		finish: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("\u21B5", "finish input"),
		),
		submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("\u21B5", "submit"),
		),
		tick: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "tick/untick"),
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

	highlightedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	depsGroupStyle = lipgloss.NewStyle().Background(lipgloss.Color("184"))

	// miscStyle = lipgloss.NewStyle().Background(lipgloss.Color("231")).Foreground(lipgloss.Color("016"))
)

func (navModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keys.help, keys.quit}
}

func (navModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keys.up, keys.down, keys.input, keys.tick},
		{keys.help, keys.quit},
	}
}

func (inputModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keys.finish, keys.quit}
}

func (inputModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keys.finish, keys.quit},
		{keys.finish, keys.quit},
	}
}

func (submitButtonKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keys.help, keys.quit}
}

func (submitButtonKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keys.up, keys.down, keys.submit},
		{keys.help, keys.quit},
	}
}

func (di *depItem) ToggleHighlight() {
	di.highlighted = !di.highlighted
}

func (di *depItem) tick() {
	di.drop = false
	di.group.drop = false
}

func (di *depItem) unTick() {
	di.drop = true

	for i := range di.group.members {
		if !di.group.members[i].drop {
			di.group.drop = false

			return
		}
	}

	di.group.drop = true
}

func (di *depItem) ToggleTick() {
	if di.drop {
		di.tick()
	} else {
		di.unTick()
	}
}

func (di *depItem) Desc() string {
	return di.desc
}

func (di *depItem) View() string {
	var b strings.Builder

	var prompt string

	if di.drop {
		prompt = "[ ] " + di.Name + ":"
	} else {
		prompt = "[x] " + di.Name + ":"
	}

	if di.highlighted {
		b.WriteString(highlightedStyle.Render(prompt))
	} else {
		b.WriteString(prompt)
	}

	b.WriteString(di.ti.View())

	return b.String()
}

func (di *depItem) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if di.drop {
		return nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, keys.input) {
		if !di.ti.Focused() {
			cmd = di.ti.Focus()

			return cmd
		}

		di.ti.Blur()

		return nil
	}

	di.ti, cmd = di.ti.Update(msg)

	return cmd
}

func (dg *depsGroup) ToggleHighlight() {
	dg.highlighted = !dg.highlighted
}

func (dg *depsGroup) tick() {
	dg.drop = false

	for i := range dg.members {
		dg.members[i].drop = false
	}
}

func (dg *depsGroup) unTick() {
	dg.drop = true

	for i := range dg.members {
		dg.members[i].drop = true
	}
}

func (dg *depsGroup) ToggleTick() {
	if dg.drop {
		dg.tick()
	} else {
		dg.unTick()
	}
}

func (dg *depsGroup) Desc() string {
	return dg.desc
}

func (dg *depsGroup) View() string {
	var (
		b      strings.Builder
		prompt string
	)

	text := lipgloss.Place(7, 1, lipgloss.Left, lipgloss.Center, depsGroupStyle.Render(dg.name))
	b.WriteString(depsGroupStyle.Render(text))

	b.WriteRune(' ')

	if dg.drop {
		prompt = "[ ]"
	} else {
		prompt = "[x]"
	}

	if dg.highlighted {
		b.WriteString(highlightedStyle.Render(prompt))
	} else {
		b.WriteString(prompt)
	}

	return b.String()
}

func (dg *depsGroup) Update(msg tea.Msg) tea.Cmd {
	return nil
}

func InitialPythonModel(cmd *scaffold.PythonProjectCmd) pythonDeps {
	groups := make([]depsGroup, 4)

	groups[0] = depsGroup{
		name:        "develop",
		highlighted: true,
	}

	groups[1] = depsGroup{name: "linting"}
	groups[2] = depsGroup{name: "test"}
	groups[3] = depsGroup{name: "docs"}

	var ti textinput.Model

	vss := cmd.VersionSetters
	depItems := make([]depItem, len(vss))

	for i := range vss {
		*vss[i].Indirect = "LATEST"

		ti = textinput.New()
		ti.Placeholder = "LATEST"
		ti.CharLimit = 128
		ti.Width = 20
		ti.Prompt = " "

		depItems[i] = depItem{
			VersionSetter: vss[i],
			ti:            ti,
			desc:          "hi there",
		}
	}

	m := pythonDeps{
		items:   make([]navItem, 0, len(depItems)+len(groups)),
		navMode: true,
		index:   0,
		cmd:     cmd,
		help:    help.New(),
	}

	grouping := [][]int{
		{0},
		{1, 2, 3},
		{4, 5, 6},
		{7, 8},
	}

	for i, items := range grouping {
		m.items = append(m.items, &groups[i])

		for _, j := range items {
			groups[i].members = append(groups[i].members, &depItems[j])
			depItems[j].group = &groups[i]
			m.items = append(m.items, &depItems[j])
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

	b.WriteString("Let's start a Python project!\n\n")

	b.WriteString("Description: ")

	if m.index < len(m.items) {
		b.WriteString(m.items[m.index].Desc())
	}

	b.WriteString("\n\n")

	for i := range m.items {
		b.WriteString(m.items[i].View())
		b.WriteRune('\n')
	}

	b.WriteRune('\n')

	if m.index == len(m.items) {
		b.WriteString(highlightedStyle.Render("[ Submit ]"))
	} else {
		b.WriteString("[ Submit ]")
	}

	b.WriteString("\n\n")

	if m.navMode && m.index == len(m.items) {
		b.WriteString(m.help.View(submitButtonKeyMap{}))
	} else if m.navMode {
		b.WriteString(m.help.View(navModeKeyMap{}))
	} else {
		b.WriteString(m.help.View(inputModeKeyMap{}))
	}

	b.WriteRune('\n')

	return b.String()
}

func (m *pythonDeps) highlightUp(index int) {
	if index < len(m.items) {
		m.items[index].ToggleHighlight()
	}

	m.items[index-1].ToggleHighlight()
}

func (m *pythonDeps) highlightDown(index int) {
	m.items[index].ToggleHighlight()

	if index+1 < len(m.items) {
		m.items[index+1].ToggleHighlight()
	}
}

func (m *pythonDeps) navModeUpdate(msg tea.KeyMsg) (cmd tea.Cmd) {
	switch {
	case key.Matches(msg, keys.up):
		if m.index > 0 {
			m.highlightUp(m.index)
			m.index -= 1
		}

		return nil
	case key.Matches(msg, keys.down):
		if m.index < len(m.items) {
			m.highlightDown(m.index)
			m.index += 1
		}

		return nil
	case key.Matches(msg, keys.input):
		di, ok := m.items[m.index].(*depItem)
		if !ok || di.drop {
			return nil
		}

		m.navMode = false
		m.help.ShowAll = false

		cmd = m.items[m.index].Update(msg)

		return cmd
	case key.Matches(msg, keys.tick):
		m.items[m.index].ToggleTick()

		return nil
	default:
		return nil
	}
}

func (m *pythonDeps) scaffoldCmd() tea.Msg {
	// for i := range m.items {
	// 	if v := m.items[i].ti.Value(); v != "" {
	// 		*m.items[i].Indirect = v
	// 	}
	// }

	// m.cmd.ProjectName = "fs-walk"
	// m.cmd.PythonVersion = scaffold.PythonVersion{Major: "3", Minor: "12"}
	// m.cmd.TimeoutSeconds = 1

	// _ = m.cmd.AfterApply()

	// _ = m.cmd.Run()
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
		case m.index == len(m.items) && key.Matches(msg, keys.submit):
			m.working = true

			return m, m.scaffoldCmd
		case m.navMode && key.Matches(msg, keys.help):
			m.help.ShowAll = !m.help.ShowAll

			return m, nil
		case m.navMode:
			cmd = m.navModeUpdate(msg)

			return m, cmd
		case !m.navMode && key.Matches(msg, keys.input):
			cmd = m.items[m.index].Update(msg)
			m.navMode = true

			return m, cmd
		case !m.navMode:
			cmd = m.items[m.index].Update(msg)

			return m, cmd
		default:
		}
	}

	cmd = m.items[m.index].Update(msg)

	return m, cmd
}
