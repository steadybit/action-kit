// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_test/client"
	"log"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type model struct {
	client  client.ActionAPI
	baseUrl url.URL
	list    list.Model
	err     error
}

type action struct {
	ref  action_kit_api.DescribingEndpointReference
	desc action_kit_api.ActionDescription
	err  error

	showSpinner bool
	spinner     spinner.Model
}

func newAction(ref action_kit_api.DescribingEndpointReference) action {
	return action{ref: ref, spinner: spinner.New()}
}

func (a action) Title() string {
	var view string
	if a.showSpinner {
		view = a.spinner.View() + " "
	}

	if a.desc.Label != "" {
		view += a.desc.Label
	} else {
		view += fmt.Sprintf("%s %s", a.ref.Method, a.ref.Path)
	}

	return view
}

func (a action) Description() string {
	if a.err != nil {
		return a.err.Error()
	} else {
		return a.desc.Id
	}
}

func (a action) FilterValue() string {
	return a.desc.Label
}

func (a *action) startSpinner() tea.Cmd {
	a.showSpinner = true
	return a.spinner.Tick
}

func (a *action) stopSpinner() {
	a.showSpinner = false
}

type actionsListedMsg struct {
	result []action_kit_api.DescribingEndpointReference
	err    error
}

type actionDescribedMsg action

type initMsg struct{}

func (m model) listActions() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1 * time.Second)
		result, err := m.client.ListActions()
		return actionsListedMsg{result: result.Actions, err: err}
	}
}

func (m model) describeAction(a action) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1 * time.Second)
		desc, err := m.client.DescribeAction(a.ref)
		return actionDescribedMsg{ref: a.ref, desc: desc, err: err}
	}
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		return initMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case initMsg:
		cmds := tea.Batch(
			m.list.StartSpinner(),
			m.listActions(),
		)
		return m, cmds

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case actionsListedMsg:
		var cmds []tea.Cmd
		items := make([]list.Item, len(msg.result))
		for i, ref := range msg.result {
			action := newAction(ref)
			cmds = append(cmds,
				action.startSpinner(),
				m.describeAction(action),
			)
			items[i] = action
		}
		m.list.StopSpinner()
		m.err = msg.err
		cmds = append(cmds, m.list.SetItems(sortItems(items)))
		return m, tea.Batch(cmds...)

	case actionDescribedMsg:
		for i, item := range m.list.Items() {
			action := item.(action)
			if action.ref == msg.ref {
				action.desc = msg.desc
				action.err = msg.err
				action.stopSpinner()
				m.list.SetItem(i, action)
				break
			}
		}
		return m, m.list.SetItems(sortItems(m.list.Items()))
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func sortItems(items []list.Item) []list.Item {
	slices.SortFunc(items, func(a, b list.Item) int {
		c := strings.Compare(a.(action).Description(), b.(action).Description())
		if c == 0 {
			c = strings.Compare(a.(action).Title(), b.(action).Title())
		}
		return c
	})
	return items
}

func (m model) View() string {
	if m.err != nil {
		return docStyle.Render(m.err.Error())
	}
	return docStyle.Render(m.list.View())
}

func newModel(baseUrl string) model {
	parsed, err := url.Parse(baseUrl)
	if err != nil {
		log.Fatalf("error parsing url: %v", err)
	}

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = fmt.Sprintf("Actions on %s", parsed.String())
	l.SetStatusBarItemName("action", "actions")
	l.SetDelegate(newItemDelegate())

	c := client.NewActionClient(parsed.Path, resty.New().SetBaseURL(parsed.String()))

	return model{baseUrl: *parsed, list: l, client: c}
}

func newItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		switch msg := msg.(type) {
		case spinner.TickMsg:
			var cmds []tea.Cmd
			for i, item := range m.Items() {
				var cmd tea.Cmd
				action := item.(action)
				action.spinner, cmd = action.spinner.Update(msg)
				cmds = append(cmds, cmd)
				m.SetItem(i, action)
			}
			return tea.Batch(cmds...)
		}
		return nil
	}
	return d
}

func main() {
	m := newModel("http://localhost:8085")
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
