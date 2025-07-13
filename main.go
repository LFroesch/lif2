package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Data structures
type Daily struct {
	ID       int    `json:"id"`
	Task     string `json:"task"`
	Priority string `json:"priority"`
	Category string `json:"category"`
	Deadline string `json:"deadline"`
	Status   string `json:"status"`
}

type RollingTodo struct {
	ID       int    `json:"id"`
	Task     string `json:"task"`
	Priority string `json:"priority"`
	Category string `json:"category"`
	Deadline string `json:"deadline"`
}

type Reminder struct {
	ID               int    `json:"id"`
	Reminder         string `json:"reminder"`
	Note             string `json:"note"`
	AlarmOrCountdown string `json:"alarm_or_countdown"`
	Status           string `json:"status"`
}

type GlossaryItem struct {
	ID      int    `json:"id"`
	Lang    string `json:"lang"`
	Command string `json:"command"`
	Usage   string `json:"usage"`
	Example string `json:"example"`
	Meaning string `json:"meaning"`
}

type AppData struct {
	Dailies      []Daily        `json:"dailies"`
	RollingTodos []RollingTodo  `json:"rolling_todos"`
	Reminders    []Reminder     `json:"reminders"`
	Glossary     []GlossaryItem `json:"glossary"`
}

type statusMsg struct {
	message string
	color   string
}

// Model
type model struct {
	activeTab    int
	tables       [4]table.Model
	data         AppData
	editing      bool
	editingTab   int
	editingRow   int
	editingField int
	inputs       []textinput.Model
	statusMsg    string
	statusColor  string
	statusExpiry time.Time
	width        int
	height       int
}

// Enhanced styles with better color coding
var (
	// Tab styles
	tabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Background(lipgloss.Color("236")).
			PaddingLeft(1).
			PaddingRight(1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			PaddingLeft(1).
			PaddingRight(1)

	// Priority color styles
	priorityHighStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // Red
	priorityMedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true) // Yellow
	priorityLowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)  // Green

	// Status color styles
	statusDoneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)  // Green
	statusPendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true) // Yellow
	statusOverdueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // Red

	// Command styles
	keyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // Blue
	actionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))  // Green
	bulletStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray

	// Header style
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))
)

func showStatus(msg string, color string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{message: msg, color: color}
	}
}

func initialModel() model {
	m := model{
		activeTab:   1,
		data:        loadData(),
		statusColor: "86",
	}

	m.setupTables()
	return m
}

func (m *model) setupTables() {
	// Tab 2: Dailies
	m.tables[0] = table.New(
		table.WithColumns([]table.Column{
			{Title: "Task", Width: 30},
			{Title: "Priority", Width: 10},
			{Title: "Category", Width: 15},
			{Title: "Deadline", Width: 12},
			{Title: "Status", Width: 25},
		}),
		table.WithRows(m.dailyRows()),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Tab 3: Rolling Todos
	m.tables[1] = table.New(
		table.WithColumns([]table.Column{
			{Title: "Task", Width: 40},
			{Title: "Priority", Width: 10},
			{Title: "Category", Width: 15},
			{Title: "Deadline", Width: 15},
		}),
		table.WithRows(m.rollingRows()),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Tab 4: Reminders
	m.tables[2] = table.New(
		table.WithColumns([]table.Column{
			{Title: "Reminder", Width: 30},
			{Title: "Note", Width: 35},
			{Title: "Alarm/Countdown", Width: 15},
			{Title: "Status", Width: 10},
		}),
		table.WithRows(m.reminderRows()),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Tab 5: Glossary
	m.tables[3] = table.New(
		table.WithColumns([]table.Column{
			{Title: "Lang", Width: 8},
			{Title: "Command", Width: 18},
			{Title: "Usage", Width: 25},
			{Title: "Example", Width: 25},
			{Title: "Meaning", Width: 25},
		}),
		table.WithRows(m.glossaryRows()),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Apply modern table styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("86"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	for i := range m.tables {
		m.tables[i].SetStyles(s)
	}
}

func (m *model) adjustLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	tableHeight := m.height - 8
	if tableHeight < 10 {
		tableHeight = 10
	}

	// Adjust table heights
	for i := range m.tables {
		m.tables[i].SetHeight(tableHeight)
	}
}

func (m *model) dailyRows() []table.Row {
	rows := []table.Row{}
	for _, daily := range m.data.Dailies {
		priority := daily.Priority
		switch priority {
		case "High":
			priority = priorityHighStyle.Render(priority)
		case "Medium":
			priority = priorityMedStyle.Render(priority)
		case "Low":
			priority = priorityLowStyle.Render(priority)
		}

		status := daily.Status
		switch status {
		case "DONE":
			status = statusDoneStyle.Render(status)
		default:
			status = statusOverdueStyle.Render("INCOMPLETE")
		}

		rows = append(rows, table.Row{
			daily.Task,
			priority,
			daily.Category,
			daily.Deadline,
			status,
		})
	}
	return rows
}

func (m *model) rollingRows() []table.Row {
	rows := []table.Row{}
	for _, todo := range m.data.RollingTodos {
		priority := todo.Priority
		switch priority {
		case "High":
			priority = priorityHighStyle.Render(priority)
		case "Medium":
			priority = priorityMedStyle.Render(priority)
		case "Low":
			priority = priorityLowStyle.Render(priority)
		}

		rows = append(rows, table.Row{
			todo.Task,
			priority,
			todo.Category,
			todo.Deadline,
		})
	}
	return rows
}

func (m *model) reminderRows() []table.Row {
	rows := []table.Row{}
	for _, reminder := range m.data.Reminders {
		status := reminder.Status
		switch status {
		case "Active":
			status = statusPendingStyle.Render(status)
		case "Completed", "Done":
			status = statusDoneStyle.Render(status)
		case "Expired":
			status = statusOverdueStyle.Render(status)
		}

		rows = append(rows, table.Row{
			reminder.Reminder,
			reminder.Note,
			reminder.AlarmOrCountdown,
			status,
		})
	}
	return rows
}

func (m *model) glossaryRows() []table.Row {
	rows := []table.Row{}
	for _, item := range m.data.Glossary {
		rows = append(rows, table.Row{
			item.Lang,
			item.Command,
			item.Usage,
			item.Example,
			item.Meaning,
		})
	}
	return rows
}

func (m *model) toggleCompletion() {
	if m.activeTab != 2 || len(m.data.Dailies) == 0 {
		return
	}

	cursor := m.tables[0].Cursor()
	if cursor >= len(m.data.Dailies) {
		return
	}

	current := m.data.Dailies[cursor].Status
	var newStatus string

	switch current {
	case "DONE":
		newStatus = "INCOMPLETE"
	default:
		newStatus = "DONE"
	}

	m.data.Dailies[cursor].Status = newStatus
	m.tables[0].SetRows(m.dailyRows())
	saveData(m.data)

	statusColor := "86"
	if newStatus == "DONE" {
		statusColor = "82"
	} else {
		statusColor = "196"
	}
	m.statusMsg = fmt.Sprintf("✅ Task marked as %s", newStatus)
	m.statusColor = statusColor
	m.statusExpiry = time.Now().Add(3 * time.Second)
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusColor = msg.color
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustLayout()
		return m, nil

	case tea.KeyMsg:
		if m.editing {
			return m.handleEditingKeys(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "1":
			m.activeTab = 1
		case "2":
			m.activeTab = 2
		case "3":
			m.activeTab = 3
		case "4":
			m.activeTab = 4
		case "5":
			m.activeTab = 5
		case "left":
			if m.activeTab > 1 {
				m.activeTab--
			} else if m.activeTab == 1 {
				m.activeTab = 5
			}
		case "right":
			if m.activeTab < 5 {
				m.activeTab++
			} else if m.activeTab == 5 {
				m.activeTab = 1
			}
		case "up", "k":
			if m.activeTab > 1 && m.activeTab < 6 {
				m.tables[m.activeTab-2], _ = m.tables[m.activeTab-2].Update(msg)
			}
		case "down", "j":
			if m.activeTab > 1 && m.activeTab < 6 {
				m.tables[m.activeTab-2], _ = m.tables[m.activeTab-2].Update(msg)
			}
		case "e":
			if m.activeTab > 1 && m.activeTab < 6 {
				m.startEditing()
			}
		case "n", "a":
			if m.activeTab > 1 && m.activeTab < 6 {
				m.addNew()
			}
		case "d", "delete":
			if m.activeTab > 1 && m.activeTab < 6 {
				m.deleteSelected()
			}
		case " ", "enter":
			// Toggle completion for dailies
			if m.activeTab == 2 {
				m.toggleCompletion()
			}

		}
	}

	return m, nil
}

func (m model) handleEditingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = false
		m.inputs = nil
		return m, showStatus("❌ Edit cancelled", "196")
	case "enter":
		m.saveEdit()
		m.editing = false
		m.inputs = nil
		return m, showStatus("✅ Changes saved", "82")
	case "tab":
		if len(m.inputs) > 0 {
			m.editingField = (m.editingField + 1) % len(m.inputs)
			for i := range m.inputs {
				m.inputs[i].Blur()
			}
			m.inputs[m.editingField].Focus()
		}
	case "shift+tab":
		if len(m.inputs) > 0 {
			m.editingField = (m.editingField - 1 + len(m.inputs)) % len(m.inputs)
			for i := range m.inputs {
				m.inputs[i].Blur()
			}
			m.inputs[m.editingField].Focus()
		}
	default:
		if len(m.inputs) > 0 {
			var cmd tea.Cmd
			m.inputs[m.editingField], cmd = m.inputs[m.editingField].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *model) startEditing() {
	m.editing = true
	m.editingTab = m.activeTab
	m.editingRow = m.tables[m.activeTab-2].Cursor()
	m.editingField = 0

	switch m.editingTab {
	case 2: // Dailies
		if m.editingRow < len(m.data.Dailies) {
			daily := m.data.Dailies[m.editingRow]
			m.inputs = make([]textinput.Model, 4)
			m.inputs[0] = textinput.New()
			m.inputs[0].SetValue(daily.Task)
			m.inputs[0].Focus()
			m.inputs[1] = textinput.New()
			m.inputs[1].SetValue(daily.Priority)
			m.inputs[2] = textinput.New()
			m.inputs[2].SetValue(daily.Category)
			m.inputs[3] = textinput.New()
			m.inputs[3].SetValue(daily.Deadline)
		}
	case 3: // Rolling Todos
		if m.editingRow < len(m.data.RollingTodos) {
			todo := m.data.RollingTodos[m.editingRow]
			m.inputs = make([]textinput.Model, 4)
			m.inputs[0] = textinput.New()
			m.inputs[0].SetValue(todo.Task)
			m.inputs[0].Focus()
			m.inputs[1] = textinput.New()
			m.inputs[1].SetValue(todo.Priority)
			m.inputs[2] = textinput.New()
			m.inputs[2].SetValue(todo.Category)
			m.inputs[3] = textinput.New()
			m.inputs[3].SetValue(todo.Deadline)
		}
	case 4: // Reminders
		if m.editingRow < len(m.data.Reminders) {
			reminder := m.data.Reminders[m.editingRow]
			m.inputs = make([]textinput.Model, 4)
			m.inputs[0] = textinput.New()
			m.inputs[0].SetValue(reminder.Reminder)
			m.inputs[0].Focus()
			m.inputs[1] = textinput.New()
			m.inputs[1].SetValue(reminder.Note)
			m.inputs[2] = textinput.New()
			m.inputs[2].SetValue(reminder.AlarmOrCountdown)
			m.inputs[3] = textinput.New()
			m.inputs[3].SetValue(reminder.Status)
		}
	case 5: // Glossary
		if m.editingRow < len(m.data.Glossary) {
			item := m.data.Glossary[m.editingRow]
			m.inputs = make([]textinput.Model, 5)
			m.inputs[0] = textinput.New()
			m.inputs[0].SetValue(item.Lang)
			m.inputs[0].Focus()
			m.inputs[1] = textinput.New()
			m.inputs[1].SetValue(item.Command)
			m.inputs[2] = textinput.New()
			m.inputs[2].SetValue(item.Usage)
			m.inputs[3] = textinput.New()
			m.inputs[3].SetValue(item.Example)
			m.inputs[4] = textinput.New()
			m.inputs[4].SetValue(item.Meaning)
		}
	}
}

func (m *model) addNew() {
	m.editing = true
	m.editingTab = m.activeTab
	m.editingRow = -1 // Indicates new item
	m.editingField = 0

	switch m.activeTab {
	case 2: // Dailies
		m.inputs = make([]textinput.Model, 4)
		for i := range m.inputs {
			m.inputs[i] = textinput.New()
		}
		m.inputs[0].Focus()
	case 3: // Rolling Todos
		m.inputs = make([]textinput.Model, 4)
		for i := range m.inputs {
			m.inputs[i] = textinput.New()
		}
		m.inputs[0].Focus()
	case 4: // Reminders
		m.inputs = make([]textinput.Model, 4)
		for i := range m.inputs {
			m.inputs[i] = textinput.New()
		}
		m.inputs[0].Focus()
	case 5: // Glossary
		m.inputs = make([]textinput.Model, 5)
		for i := range m.inputs {
			m.inputs[i] = textinput.New()
		}
		m.inputs[0].Focus()
	}
}

func (m *model) saveEdit() {
	switch m.editingTab {
	case 2: // Dailies
		if m.editingRow == -1 {
			// New item
			newDaily := Daily{
				ID:       len(m.data.Dailies) + 1,
				Task:     m.inputs[0].Value(),
				Priority: m.inputs[1].Value(),
				Category: m.inputs[2].Value(),
				Deadline: m.inputs[3].Value(),
				Status:   "INCOMPLETE",
			}
			m.data.Dailies = append(m.data.Dailies, newDaily)
		} else {
			// Edit existing
			m.data.Dailies[m.editingRow].Task = m.inputs[0].Value()
			m.data.Dailies[m.editingRow].Priority = m.inputs[1].Value()
			m.data.Dailies[m.editingRow].Category = m.inputs[2].Value()
			m.data.Dailies[m.editingRow].Deadline = m.inputs[3].Value()
		}
		m.tables[0].SetRows(m.dailyRows())
	case 3: // Rolling Todos
		if m.editingRow == -1 {
			newTodo := RollingTodo{
				ID:       len(m.data.RollingTodos) + 1,
				Task:     m.inputs[0].Value(),
				Priority: m.inputs[1].Value(),
				Category: m.inputs[2].Value(),
				Deadline: m.inputs[3].Value(),
			}
			m.data.RollingTodos = append(m.data.RollingTodos, newTodo)
		} else {
			m.data.RollingTodos[m.editingRow].Task = m.inputs[0].Value()
			m.data.RollingTodos[m.editingRow].Priority = m.inputs[1].Value()
			m.data.RollingTodos[m.editingRow].Category = m.inputs[2].Value()
			m.data.RollingTodos[m.editingRow].Deadline = m.inputs[3].Value()
		}
		m.tables[1].SetRows(m.rollingRows())
	case 4: // Reminders
		if m.editingRow == -1 {
			newReminder := Reminder{
				ID:               len(m.data.Reminders) + 1,
				Reminder:         m.inputs[0].Value(),
				Note:             m.inputs[1].Value(),
				AlarmOrCountdown: m.inputs[2].Value(),
				Status:           m.inputs[3].Value(),
			}
			m.data.Reminders = append(m.data.Reminders, newReminder)
		} else {
			m.data.Reminders[m.editingRow].Reminder = m.inputs[0].Value()
			m.data.Reminders[m.editingRow].Note = m.inputs[1].Value()
			m.data.Reminders[m.editingRow].AlarmOrCountdown = m.inputs[2].Value()
			m.data.Reminders[m.editingRow].Status = m.inputs[3].Value()
		}
		m.tables[2].SetRows(m.reminderRows())
	case 5: // Glossary
		if m.editingRow == -1 {
			newItem := GlossaryItem{
				ID:      len(m.data.Glossary) + 1,
				Lang:    m.inputs[0].Value(),
				Command: m.inputs[1].Value(),
				Usage:   m.inputs[2].Value(),
				Example: m.inputs[3].Value(),
				Meaning: m.inputs[4].Value(),
			}
			m.data.Glossary = append(m.data.Glossary, newItem)
		} else {
			m.data.Glossary[m.editingRow].Lang = m.inputs[0].Value()
			m.data.Glossary[m.editingRow].Command = m.inputs[1].Value()
			m.data.Glossary[m.editingRow].Usage = m.inputs[2].Value()
			m.data.Glossary[m.editingRow].Example = m.inputs[3].Value()
			m.data.Glossary[m.editingRow].Meaning = m.inputs[4].Value()
		}
		m.tables[3].SetRows(m.glossaryRows())
	}

	saveData(m.data)
}

func (m *model) deleteSelected() {
	cursor := m.tables[m.activeTab-2].Cursor()

	switch m.activeTab {
	case 2: // Dailies
		if cursor < len(m.data.Dailies) {
			taskName := m.data.Dailies[cursor].Task
			m.data.Dailies = append(m.data.Dailies[:cursor], m.data.Dailies[cursor+1:]...)
			m.tables[0].SetRows(m.dailyRows())
			m.statusMsg = fmt.Sprintf("🗑️ Deleted: %s", taskName)
			m.statusColor = "196"
			m.statusExpiry = time.Now().Add(3 * time.Second)
		}
	case 3: // Rolling Todos
		if cursor < len(m.data.RollingTodos) {
			taskName := m.data.RollingTodos[cursor].Task
			m.data.RollingTodos = append(m.data.RollingTodos[:cursor], m.data.RollingTodos[cursor+1:]...)
			m.tables[1].SetRows(m.rollingRows())
			m.statusMsg = fmt.Sprintf("🗑️ Deleted: %s", taskName)
			m.statusColor = "196"
			m.statusExpiry = time.Now().Add(3 * time.Second)
		}
	case 4: // Reminders
		if cursor < len(m.data.Reminders) {
			reminderName := m.data.Reminders[cursor].Reminder
			m.data.Reminders = append(m.data.Reminders[:cursor], m.data.Reminders[cursor+1:]...)
			m.tables[2].SetRows(m.reminderRows())
			m.statusMsg = fmt.Sprintf("🗑️ Deleted: %s", reminderName)
			m.statusColor = "196"
			m.statusExpiry = time.Now().Add(3 * time.Second)
		}
	case 5: // Glossary
		if cursor < len(m.data.Glossary) {
			itemName := m.data.Glossary[cursor].Command
			m.data.Glossary = append(m.data.Glossary[:cursor], m.data.Glossary[cursor+1:]...)
			m.tables[3].SetRows(m.glossaryRows())
			m.statusMsg = fmt.Sprintf("🗑️ Deleted: %s", itemName)
			m.statusColor = "196"
			m.statusExpiry = time.Now().Add(3 * time.Second)
		}
	}

	saveData(m.data)
}

func (m model) View() string {
	if m.editing {
		return m.editView()
	}

	// Header
	header := headerStyle.Render("📋 lif2 - lucas is forgetful")

	// Tab headers
	tabs := []string{}
	tabNames := []string{"[1] Home", "[2] Dailies", "[3] Rolling", "[4] Reminders", "[5] Glossary"}

	for i, name := range tabNames {
		if i+1 == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(name))
		} else {
			tabs = append(tabs, tabStyle.Render(name))
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	var content string

	if m.activeTab == 1 {
		// Enhanced home page with summary
		homeContent := lipgloss.NewStyle().
			Padding(1).
			Render("Welcome to the LIF Dailies/Reminders Management Dashboard!")

		// Show summary stats
		totalDailies := len(m.data.Dailies)
		completedDailies := 0
		for _, daily := range m.data.Dailies {
			if daily.Status == "DONE" {
				completedDailies++
			}
		}
		summary := fmt.Sprintf("\nStats:\n")
		summary += fmt.Sprintf("  • Daily Tasks: %d total, %d completed\n", totalDailies, completedDailies)
		summary += fmt.Sprintf("  • Rolling Todos: %d items\n", len(m.data.RollingTodos))
		summary += fmt.Sprintf("  • Reminders: %d active\n", len(m.data.Reminders))
		summary += fmt.Sprintf("  • Glossary: %d entries\n", len(m.data.Glossary))

		if len(m.data.RollingTodos) > 0 {
			summary += "\n" + priorityHighStyle.Render("You have items in your Rolling Todo List!")
		}

		content = homeContent + summary
	} else {
		// Table content
		content = m.tables[m.activeTab-2].View()
	}

	// Enhanced footer with color coding
	var commands []string
	if m.activeTab == 1 {
		commands = append(commands, keyStyle.Render("1-5")+": "+actionStyle.Render("navigate"))
	} else {
		commands = append(commands, keyStyle.Render("↑↓")+": "+actionStyle.Render("navigate"))
		commands = append(commands, keyStyle.Render("e")+": "+actionStyle.Render("edit"))
		commands = append(commands, keyStyle.Render("n/a")+": "+actionStyle.Render("add"))
		commands = append(commands, keyStyle.Render("d")+": "+actionStyle.Render("delete"))
		if m.activeTab == 2 {
			commands = append(commands, keyStyle.Render("space/enter")+": "+actionStyle.Render("toggle done"))
		}
	}
	commands = append(commands, keyStyle.Render("q")+": "+actionStyle.Render("quit"))

	commandRow := strings.Join(commands, bulletStyle.Render(" • "))

	// Status message with expiry
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.statusColor))
		commandRow += "\n> " + statusStyle.Render(m.statusMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		header,
		"",
		tabRow,
		content,
		"",
		commandRow,
	)
}

func (m model) editView() string {
	var fields []string
	var labels []string

	switch m.editingTab {
	case 2: // Dailies
		labels = []string{"Task:", "Priority:", "Category:", "Deadline:", "Status:"}
	case 3: // Rolling Todos
		labels = []string{"Task:", "Priority:", "Category:", "Deadline:"}
	case 4: // Reminders
		labels = []string{"Reminder:", "Note:", "Alarm/Countdown:", "Status:"}
	case 5: // Glossary
		labels = []string{"Lang:", "Command:", "Usage:", "Example:", "Meaning:"}
	}

	for i, input := range m.inputs {
		label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render(labels[i])
		fields = append(fields, label+"\n"+input.View())
	}

	content := lipgloss.JoinVertical(lipgloss.Top, fields...)

	header := headerStyle.Render("✏️ Editing Mode")
	footer := keyStyle.Render("tab") + ": " + actionStyle.Render("next field") + " " + bulletStyle.Render("•") + " " + keyStyle.Render("shift+tab") + ": " + actionStyle.Render("prev field") + " " + bulletStyle.Render("•") + " " + keyStyle.Render("enter") + ": " + actionStyle.Render("save") + " " + bulletStyle.Render("•") + " " + keyStyle.Render("esc") + ": " + actionStyle.Render("cancel")

	return lipgloss.JoinVertical(lipgloss.Top,
		header,
		"",
		content,
		"",
		footer,
	)
}

func loadData() AppData {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	configPath := filepath.Join(configDir, "daily-tasks", "config.json")

	// Create directory if it doesn't exist
	os.MkdirAll(filepath.Dir(configPath), 0755)

	data := AppData{
		Dailies:      []Daily{},
		RollingTodos: []RollingTodo{},
		Reminders:    []Reminder{},
		Glossary:     []GlossaryItem{},
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		saveData(data)
		return data
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	json.Unmarshal(file, &data)
	return data
}

func saveData(data AppData) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	configPath := filepath.Join(configDir, "daily-tasks", "config.json")

	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(configPath, file, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
