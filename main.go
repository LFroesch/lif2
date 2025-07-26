package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Data structures
type Daily struct {
	ID            int       `json:"id"`
	Task          string    `json:"task"`
	Priority      string    `json:"priority"`
	Category      string    `json:"category"`
	Deadline      string    `json:"deadline"`
	Status        string    `json:"status"`
	LastCompleted time.Time `json:"last_completed"`
}

type RollingTodo struct {
	ID       int    `json:"id"`
	Task     string `json:"task"`
	Priority string `json:"priority"`
	Category string `json:"category"`
	Deadline string `json:"deadline"`
}

type Reminder struct {
	ID               int           `json:"id"`
	Reminder         string        `json:"reminder"`
	Note             string        `json:"note"`
	AlarmOrCountdown string        `json:"alarm_or_countdown"`
	Status           string        `json:"status"`
	CreatedAt        time.Time     `json:"created_at"`
	TargetTime       time.Time     `json:"target_time"`
	IsCountdown      bool          `json:"is_countdown"`
	Notified         bool          `json:"notified"`
	PausedRemaining  time.Duration `json:"paused_remaining"`
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

type tickMsg time.Time

type notificationMsg struct {
	reminder Reminder
}

// Model
type model struct {
	activeTab     int
	tables        [4]table.Model
	data          AppData
	editing       bool
	editingTab    int
	editingRow    int
	editingField  int
	inputs        []textinput.Model
	statusMsg     string
	statusColor   string
	statusExpiry  time.Time
	width         int
	height        int
	lastTick      time.Time
	confirmDelete bool
	deleteTarget  string
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

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func normalizeText(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func normalizePriority(priority string) string {
	norm := strings.ToUpper(strings.TrimSpace(priority))
	switch norm {
	case "HIGH", "H":
		return "HIGH"
	case "MEDIUM", "MED", "M":
		return "MEDIUM"
	case "LOW", "L":
		return "LOW"
	default:
		// Handle legacy values
		lower := strings.ToLower(norm)
		if strings.Contains(lower, "high") {
			return "HIGH"
		} else if strings.Contains(lower, "low") {
			return "LOW"
		}
		return "MEDIUM"
	}
}

func parseCountdown(countdownStr string) (time.Time, bool) {
	// Days format (1d, 5d, 20d)
	if strings.HasSuffix(countdownStr, "d") {
		dayStr := strings.TrimSuffix(countdownStr, "d")
		if days, err := strconv.Atoi(dayStr); err == nil {
			return time.Now().Add(time.Duration(days) * 24 * time.Hour), true
		}
	}

	// Weeks format (1w, 2w)
	if strings.HasSuffix(countdownStr, "w") {
		weekStr := strings.TrimSuffix(countdownStr, "w")
		if weeks, err := strconv.Atoi(weekStr); err == nil {
			return time.Now().Add(time.Duration(weeks) * 7 * 24 * time.Hour), true
		}
	}

	// Minutes format (1m, 30m, min)
	if strings.HasSuffix(countdownStr, "m") || strings.HasSuffix(countdownStr, "min") {
		minStr := strings.TrimSuffix(strings.TrimSuffix(countdownStr, "min"), "m")
		if minutes, err := strconv.Atoi(minStr); err == nil {
			return time.Now().Add(time.Duration(minutes) * time.Minute), true
		}
	}

	// Hours format (1h, 2h, hr)
	if strings.HasSuffix(countdownStr, "h") || strings.HasSuffix(countdownStr, "hr") {
		hourStr := strings.TrimSuffix(strings.TrimSuffix(countdownStr, "hr"), "h")
		if hours, err := strconv.Atoi(hourStr); err == nil {
			return time.Now().Add(time.Duration(hours) * time.Hour), true
		}
	}

	// Seconds format (1s, 30s, sec)
	if strings.HasSuffix(countdownStr, "s") || strings.HasSuffix(countdownStr, "sec") {
		secStr := strings.TrimSuffix(strings.TrimSuffix(countdownStr, "sec"), "s")
		if seconds, err := strconv.Atoi(secStr); err == nil {
			return time.Now().Add(time.Duration(seconds) * time.Second), true
		}
	}

	return time.Time{}, false
}

func parseAlarmTime(alarmStr string) (time.Time, bool) {
	now := time.Now()

	// Try 12-hour format first (1:50PM, 1:50 PM, 1:50pm, etc.)
	formats12 := []string{"3:04PM", "3:04 PM", "3:04pm", "3:04 pm"}
	for _, format := range formats12 {
		if t, err := time.Parse(format, alarmStr); err == nil {
			alarmTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
			if alarmTime.Before(now) {
				alarmTime = alarmTime.Add(24 * time.Hour)
			}
			return alarmTime, true
		}
	}

	// Try 24-hour format (15:04)
	if t, err := time.Parse("15:04", alarmStr); err == nil {
		alarmTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		if alarmTime.Before(now) {
			alarmTime = alarmTime.Add(24 * time.Hour)
		}
		return alarmTime, true
	}

	return time.Time{}, false
}

func formatDuration(d time.Duration) string {
	// If over 8 hours, round to nearest hour
	if d > 8*time.Hour {
		hours := d.Round(time.Hour)
		if hours >= 24*time.Hour {
			days := int(hours / (24 * time.Hour))
			remaining := hours % (24 * time.Hour)
			if remaining == 0 {
				if days == 1 {
					return "1 day"
				}
				return fmt.Sprintf("%d days", days)
			} else {
				hours := int(remaining / time.Hour)
				if days == 1 {
					return fmt.Sprintf("1 day %dh", hours)
				}
				return fmt.Sprintf("%dd %dh", days, hours)
			}
		} else {
			hours := int(d.Round(time.Hour) / time.Hour)
			if hours == 1 {
				return "1 hour"
			}
			return fmt.Sprintf("%d hours", hours)
		}
	}

	// For under 8 hours, show precise time
	return d.Truncate(time.Second).String()
}

func isWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	// Check if we're in WSL by looking for WSL-specific environment variables or files
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSLENV") != "" {
		return true
	}
	// Check for WSL filesystem marker
	if _, err := os.Stat("/proc/version"); err == nil {
		if data, err := os.ReadFile("/proc/version"); err == nil {
			return strings.Contains(string(data), "microsoft") || strings.Contains(string(data), "WSL")
		}
	}
	return false
}

func playNotificationSound() {
	// Try both mp3 and wav files
	soundFiles := []string{"assets/notification.mp3", "assets/notification.wav"}
	var soundFile string
	for _, file := range soundFiles {
		if _, err := os.Stat(file); err == nil {
			soundFile = file
			break
		}
	}

	// If no sound file exists, play system beep
	if soundFile == "" {
		if isWSL() {
			go exec.Command("powershell.exe", "-Command", "[console]::beep(800,200)").Run()
		} else {
			go exec.Command("printf", "\a").Run()
		}
		return
	}

	if isWSL() {
		// In WSL, just use Linux audio players if available
		players := [][]string{
			{"mpv", "--no-video", "--really-quiet", "--audio-buffer=1.0", soundFile},
			{"vlc", "--intf", "dummy", "--play-and-exit", soundFile},
			{"mplayer", "-really-quiet", soundFile},
			{"ffplay", "-nodisp", "-autoexit", "-v", "quiet", soundFile},
		}
		for _, cmd := range players {
			if _, err := exec.LookPath(cmd[0]); err == nil {
				go exec.Command(cmd[0], cmd[1:]...).Run()
				return
			}
		}
		// If no players available, just beep
		go exec.Command("powershell.exe", "-Command", "[console]::beep(800,200)").Run()
		return
	}

	switch runtime.GOOS {
	case "linux":
		// Try different audio players (in order of preference)
		players := [][]string{
			{"mpv", "--no-video", "--really-quiet", "--audio-buffer=1.0", soundFile},
			{"vlc", "--intf", "dummy", "--play-and-exit", soundFile},
			{"mplayer", "-really-quiet", soundFile},
			{"ffplay", "-nodisp", "-autoexit", "-v", "quiet", soundFile},
		}
		for _, cmd := range players {
			if _, err := exec.LookPath(cmd[0]); err == nil {
				go exec.Command(cmd[0], cmd[1:]...).Run()
				return
			}
		}
	case "darwin":
		// Use afplay on macOS
		go exec.Command("afplay", soundFile).Run()
	case "windows":
		// Use PowerShell to play sound on Windows
		go exec.Command("powershell", "-Command", fmt.Sprintf(`(New-Object Media.SoundPlayer "%s").PlaySync()`, soundFile)).Run()
	}
}

func sendNotification(title, message string) {
	// Play notification sound
	playNotificationSound()

	// Send system notification
	switch runtime.GOOS {
	case "linux":
		exec.Command("notify-send", title, message).Run()
	case "darwin":
		exec.Command("osascript", "-e", fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)).Run()
	case "windows":
		exec.Command("powershell", "-Command", fmt.Sprintf(`[System.Reflection.Assembly]::LoadWithPartialName('System.Windows.Forms'); [System.Windows.Forms.MessageBox]::Show('%s', '%s')`, message, title)).Run()
	}
}

func getMostRecent3AM() time.Time {
	now := time.Now()
	today3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())

	// If current time is before 3AM today, use yesterday's 3AM
	if now.Before(today3AM) {
		return today3AM.AddDate(0, 0, -1)
	}

	// If current time is after 3AM today, use today's 3AM
	return today3AM
}

func resetDailyTasks(data *AppData) bool {
	mostRecent3AM := getMostRecent3AM()
	resetOccurred := false

	for i := range data.Dailies {
		daily := &data.Dailies[i]
		// Reset to INCOMPLETE if task was completed before the most recent 3AM
		if daily.Status == "DONE" && daily.LastCompleted.Before(mostRecent3AM) {
			daily.Status = "INCOMPLETE"
			daily.LastCompleted = time.Time{} // Reset completion time
			resetOccurred = true
		}
	}

	return resetOccurred
}

func sortItems(items interface{}, sortBy string) {
	switch v := items.(type) {
	case []Daily:
		sort.Slice(v, func(i, j int) bool {
			if v[i].Category != v[j].Category {
				return strings.ToLower(v[i].Category) < strings.ToLower(v[j].Category)
			}
			pri := map[string]int{"HIGH": 0, "MEDIUM": 1, "LOW": 2}
			if pri[v[i].Priority] != pri[v[j].Priority] {
				return pri[v[i].Priority] < pri[v[j].Priority]
			}
			return v[i].Task < v[j].Task
		})
	case []RollingTodo:
		sort.Slice(v, func(i, j int) bool {
			if v[i].Category != v[j].Category {
				return strings.ToLower(v[i].Category) < strings.ToLower(v[j].Category)
			}
			pri := map[string]int{"HIGH": 0, "MEDIUM": 1, "LOW": 2}
			if pri[v[i].Priority] != pri[v[j].Priority] {
				return pri[v[i].Priority] < pri[v[j].Priority]
			}
			return v[i].Task < v[j].Task
		})
	case []Reminder:
		sort.Slice(v, func(i, j int) bool {
			if v[i].Status != v[j].Status {
				statusOrder := map[string]int{"active": 0, "pending": 1, "completed": 2, "expired": 3}
				return statusOrder[v[i].Status] < statusOrder[v[j].Status]
			}
			return v[i].Reminder < v[j].Reminder
		})
	case []GlossaryItem:
		sort.Slice(v, func(i, j int) bool {
			if v[i].Lang != v[j].Lang {
				return v[i].Lang < v[j].Lang
			}
			return v[i].Command < v[j].Command
		})
	}
}

func initialModel() model {
	m := model{
		activeTab:   1,
		data:        loadData(),
		statusColor: "86",
		lastTick:    time.Now(),
	}

	// Check for daily task reset on startup
	if resetDailyTasks(&m.data) {
		saveData(m.data)
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
			{Title: "Alarm/Countdown", Width: 35},
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
	sortItems(m.data.Dailies, "category")
	for _, daily := range m.data.Dailies {
		priority := daily.Priority
		if priority == "" {
			priority = "MEDIUM"
		}
		priority = strings.ToUpper(priority)

		// Just use plain text for now to test
		var displayPriority string
		switch priority {
		case "HIGH":
			displayPriority = "HIGH"
		case "MEDIUM":
			displayPriority = "MEDIUM"
		case "LOW":
			displayPriority = "LOW"
		default:
			displayPriority = "MEDIUM"
		}

		status := daily.Status
		switch status {
		case "DONE":
			status = statusDoneStyle.Render(status)
		default:
			status = statusOverdueStyle.Render("INCOMPLETE")
		}

		rows = append(rows, table.Row{
			normalizeText(daily.Task),
			displayPriority,
			normalizeText(daily.Category),
			daily.Deadline,
			status,
		})
	}
	return rows
}

func (m *model) rollingRows() []table.Row {
	rows := []table.Row{}
	sortItems(m.data.RollingTodos, "category")
	for _, todo := range m.data.RollingTodos {
		priority := todo.Priority
		if priority == "" {
			priority = "MEDIUM"
		}
		priority = strings.ToUpper(priority)

		// Just use plain text for now to test
		var displayPriority string
		switch priority {
		case "HIGH":
			displayPriority = "HIGH"
		case "MEDIUM":
			displayPriority = "MEDIUM"
		case "LOW":
			displayPriority = "LOW"
		default:
			displayPriority = "MEDIUM"
		}

		rows = append(rows, table.Row{
			normalizeText(todo.Task),
			displayPriority,
			normalizeText(todo.Category),
			todo.Deadline,
		})
	}
	return rows
}

func (m *model) reminderRows() []table.Row {
	rows := []table.Row{}
	sortItems(m.data.Reminders, "status")
	for _, reminder := range m.data.Reminders {
		// Display countdown/alarm time
		displayTime := reminder.AlarmOrCountdown
		if reminder.Status == "paused" && reminder.PausedRemaining > 0 {
			// Show paused remaining time
			if reminder.IsCountdown {
				displayTime = fmt.Sprintf("%s (PAUSED %s)", reminder.AlarmOrCountdown, reminder.PausedRemaining.Truncate(time.Second))
			} else {
				displayTime = fmt.Sprintf("%s (PAUSED)", reminder.AlarmOrCountdown)
			}
		} else if !reminder.TargetTime.IsZero() {
			remaining := time.Until(reminder.TargetTime)
			if remaining > 0 {
				if reminder.IsCountdown {
					displayTime = fmt.Sprintf("%s (%s)", reminder.AlarmOrCountdown, remaining.Truncate(time.Second))
				} else {
					displayTime = fmt.Sprintf("%s (%s)", reminder.AlarmOrCountdown, reminder.TargetTime.Format("15:04"))
				}
			} else {
				displayTime = fmt.Sprintf("%s (EXPIRED)", reminder.AlarmOrCountdown)
			}
		}

		rows = append(rows, table.Row{
			normalizeText(reminder.Reminder),
			normalizeText(reminder.Note),
			displayTime,
		})
	}
	return rows
}

func (m *model) glossaryRows() []table.Row {
	rows := []table.Row{}
	sortItems(m.data.Glossary, "lang")
	for _, item := range m.data.Glossary {
		rows = append(rows, table.Row{
			normalizeText(item.Lang),
			normalizeText(item.Command),
			normalizeText(item.Usage),
			normalizeText(item.Example),
			normalizeText(item.Meaning),
		})
	}
	return rows
}

func (m *model) toggleReminderStatus(action string) {
	if m.activeTab != 4 || len(m.data.Reminders) == 0 {
		return
	}

	cursor := m.tables[2].Cursor()
	if cursor >= len(m.data.Reminders) {
		return
	}

	reminder := &m.data.Reminders[cursor]
	var statusMsg string
	var statusColor string

	switch action {
	case "start":
		if reminder.Status == "paused" {
			// Resume from paused state
			if reminder.PausedRemaining > 0 {
				reminder.TargetTime = time.Now().Add(reminder.PausedRemaining)
				reminder.PausedRemaining = 0
			}
			reminder.Status = "active"
			reminder.Notified = false
			statusMsg = fmt.Sprintf("▶️ Resumed: %s", reminder.Reminder)
			statusColor = "82"
		} else if reminder.Status == "inactive" {
			reminder.Status = "active"
			reminder.Notified = false
			// Re-parse the alarm/countdown
			if targetTime, isCountdown := parseCountdown(reminder.AlarmOrCountdown); isCountdown {
				reminder.TargetTime = targetTime
				reminder.IsCountdown = true
			} else if targetTime, isAlarm := parseAlarmTime(reminder.AlarmOrCountdown); isAlarm {
				reminder.TargetTime = targetTime
				reminder.IsCountdown = false
			}
			statusMsg = fmt.Sprintf("▶️ Started: %s", reminder.Reminder)
			statusColor = "82"
		} else {
			statusMsg = fmt.Sprintf("⚠️ %s is already active", reminder.Reminder)
			statusColor = "226"
		}

	case "pause":
		if reminder.Status == "active" {
			// Store remaining time when pausing
			if !reminder.TargetTime.IsZero() {
				reminder.PausedRemaining = time.Until(reminder.TargetTime)
				if reminder.PausedRemaining < 0 {
					reminder.PausedRemaining = 0
				}
			}
			reminder.Status = "paused"
			statusMsg = fmt.Sprintf("⏸️ Paused: %s", reminder.Reminder)
			statusColor = "226"
		} else {
			statusMsg = fmt.Sprintf("⚠️ %s is not active", reminder.Reminder)
			statusColor = "226"
		}

	case "reset":
		reminder.Status = "active"
		reminder.Notified = false
		reminder.PausedRemaining = 0 // Clear any paused time
		// Re-parse and reset the target time
		if targetTime, isCountdown := parseCountdown(reminder.AlarmOrCountdown); isCountdown {
			reminder.TargetTime = targetTime
			reminder.IsCountdown = true
		} else if targetTime, isAlarm := parseAlarmTime(reminder.AlarmOrCountdown); isAlarm {
			reminder.TargetTime = targetTime
			reminder.IsCountdown = false
		}
		statusMsg = fmt.Sprintf("🔄 Reset: %s", reminder.Reminder)
		statusColor = "82"
	}

	m.tables[2].SetRows(m.reminderRows())
	saveData(m.data)
	m.statusMsg = statusMsg
	m.statusColor = statusColor
	m.statusExpiry = time.Now().Add(3 * time.Second)
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
		m.data.Dailies[cursor].LastCompleted = time.Time{} // Clear completion time
	default:
		newStatus = "DONE"
		m.data.Dailies[cursor].LastCompleted = time.Now() // Record completion time
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
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusColor = msg.color
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case tickMsg:
		m.lastTick = time.Time(msg)

		// Check for daily task reset (runs every tick but only resets when needed)
		if resetDailyTasks(&m.data) {
			m.tables[0].SetRows(m.dailyRows())
			m.statusMsg = "🌅 Daily tasks reset at 3AM"
			m.statusColor = "82"
			m.statusExpiry = time.Now().Add(5 * time.Second)
			saveData(m.data)
		}

		// Check for reminder notifications (only for active reminders)
		for i, reminder := range m.data.Reminders {
			if !reminder.TargetTime.IsZero() && !reminder.Notified && reminder.Status == "active" && time.Now().After(reminder.TargetTime) {
				m.data.Reminders[i].Notified = true
				m.data.Reminders[i].Status = "expired"
				sendNotification("Reminder", reminder.Reminder)
				m.statusMsg = fmt.Sprintf("🔔 Reminder: %s", reminder.Reminder)
				m.statusColor = "226"
				m.statusExpiry = time.Now().Add(5 * time.Second)
				saveData(m.data)
			}
		}
		m.tables[2].SetRows(m.reminderRows())
		return m, tickCmd()

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
		case "n":
			if m.confirmDelete {
				m.confirmDelete = false
				m.deleteTarget = ""
				m.statusMsg = "Delete cancelled"
				m.statusColor = "86"
				m.statusExpiry = time.Now().Add(2 * time.Second)
			} else if m.activeTab > 1 && m.activeTab < 6 {
				m.addNew()
			}
		case "a":
			if m.activeTab > 1 && m.activeTab < 6 {
				m.addNew()
			}
		case "d", "delete":
			if m.activeTab > 1 && m.activeTab < 6 && !m.confirmDelete {
				m.confirmDeleteSelected()
			}
		case "y":
			if m.confirmDelete {
				m.deleteSelected()
				m.confirmDelete = false
				m.deleteTarget = ""
			}
		case "s":
			if m.activeTab == 4 {
				m.toggleReminderStatus("start")
			}
		case "p":
			if m.activeTab == 4 {
				m.toggleReminderStatus("pause")
			}
		case "r":
			if m.activeTab == 4 {
				m.toggleReminderStatus("reset")
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
			m.inputs = make([]textinput.Model, 3)
			m.inputs[0] = textinput.New()
			m.inputs[0].SetValue(reminder.Reminder)
			m.inputs[0].Focus()
			m.inputs[1] = textinput.New()
			m.inputs[1].SetValue(reminder.Note)
			m.inputs[2] = textinput.New()
			m.inputs[2].SetValue(reminder.AlarmOrCountdown)
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
		m.inputs = make([]textinput.Model, 3)
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
				ID:            len(m.data.Dailies) + 1,
				Task:          normalizeText(m.inputs[0].Value()),
				Priority:      normalizePriority(m.inputs[1].Value()),
				Category:      normalizeText(m.inputs[2].Value()),
				Deadline:      m.inputs[3].Value(),
				Status:        "INCOMPLETE",
				LastCompleted: time.Time{},
			}
			m.data.Dailies = append(m.data.Dailies, newDaily)
		} else {
			// Edit existing
			m.data.Dailies[m.editingRow].Task = normalizeText(m.inputs[0].Value())
			m.data.Dailies[m.editingRow].Priority = normalizePriority(m.inputs[1].Value())
			m.data.Dailies[m.editingRow].Category = normalizeText(m.inputs[2].Value())
			m.data.Dailies[m.editingRow].Deadline = m.inputs[3].Value()
		}
		m.tables[0].SetRows(m.dailyRows())
	case 3: // Rolling Todos
		if m.editingRow == -1 {
			newTodo := RollingTodo{
				ID:       len(m.data.RollingTodos) + 1,
				Task:     normalizeText(m.inputs[0].Value()),
				Priority: normalizePriority(m.inputs[1].Value()),
				Category: normalizeText(m.inputs[2].Value()),
				Deadline: m.inputs[3].Value(),
			}
			m.data.RollingTodos = append(m.data.RollingTodos, newTodo)
		} else {
			m.data.RollingTodos[m.editingRow].Task = normalizeText(m.inputs[0].Value())
			m.data.RollingTodos[m.editingRow].Priority = normalizePriority(m.inputs[1].Value())
			m.data.RollingTodos[m.editingRow].Category = normalizeText(m.inputs[2].Value())
			m.data.RollingTodos[m.editingRow].Deadline = m.inputs[3].Value()
		}
		m.tables[1].SetRows(m.rollingRows())
	case 4: // Reminders
		if m.editingRow == -1 {
			newReminder := Reminder{
				ID:               len(m.data.Reminders) + 1,
				Reminder:         normalizeText(m.inputs[0].Value()),
				Note:             normalizeText(m.inputs[1].Value()),
				AlarmOrCountdown: m.inputs[2].Value(),
				CreatedAt:        time.Now(),
				Notified:         false,
			}
			// Parse countdown or alarm
			if targetTime, isCountdown := parseCountdown(m.inputs[2].Value()); isCountdown {
				newReminder.TargetTime = targetTime
				newReminder.IsCountdown = true
				newReminder.Status = "active"
			} else if targetTime, isAlarm := parseAlarmTime(m.inputs[2].Value()); isAlarm {
				newReminder.TargetTime = targetTime
				newReminder.IsCountdown = false
				newReminder.Status = "active"
			}
			m.data.Reminders = append(m.data.Reminders, newReminder)
		} else {
			m.data.Reminders[m.editingRow].Reminder = normalizeText(m.inputs[0].Value())
			m.data.Reminders[m.editingRow].Note = normalizeText(m.inputs[1].Value())
			m.data.Reminders[m.editingRow].AlarmOrCountdown = m.inputs[2].Value()
			// Re-parse countdown or alarm when editing
			if targetTime, isCountdown := parseCountdown(m.inputs[2].Value()); isCountdown {
				m.data.Reminders[m.editingRow].TargetTime = targetTime
				m.data.Reminders[m.editingRow].IsCountdown = true
				m.data.Reminders[m.editingRow].Notified = false
				m.data.Reminders[m.editingRow].Status = "active"
			} else if targetTime, isAlarm := parseAlarmTime(m.inputs[2].Value()); isAlarm {
				m.data.Reminders[m.editingRow].TargetTime = targetTime
				m.data.Reminders[m.editingRow].IsCountdown = false
				m.data.Reminders[m.editingRow].Notified = false
				m.data.Reminders[m.editingRow].Status = "active"
			}
		}
		m.tables[2].SetRows(m.reminderRows())
	case 5: // Glossary
		if m.editingRow == -1 {
			newItem := GlossaryItem{
				ID:      len(m.data.Glossary) + 1,
				Lang:    normalizeText(m.inputs[0].Value()),
				Command: normalizeText(m.inputs[1].Value()),
				Usage:   normalizeText(m.inputs[2].Value()),
				Example: normalizeText(m.inputs[3].Value()),
				Meaning: normalizeText(m.inputs[4].Value()),
			}
			m.data.Glossary = append(m.data.Glossary, newItem)
		} else {
			m.data.Glossary[m.editingRow].Lang = normalizeText(m.inputs[0].Value())
			m.data.Glossary[m.editingRow].Command = normalizeText(m.inputs[1].Value())
			m.data.Glossary[m.editingRow].Usage = normalizeText(m.inputs[2].Value())
			m.data.Glossary[m.editingRow].Example = normalizeText(m.inputs[3].Value())
			m.data.Glossary[m.editingRow].Meaning = normalizeText(m.inputs[4].Value())
		}
		m.tables[3].SetRows(m.glossaryRows())
	}

	saveData(m.data)
}

func (m *model) confirmDeleteSelected() {
	cursor := m.tables[m.activeTab-2].Cursor()
	var itemName string

	switch m.activeTab {
	case 2: // Dailies
		if cursor < len(m.data.Dailies) {
			itemName = m.data.Dailies[cursor].Task
		}
	case 3: // Rolling Todos
		if cursor < len(m.data.RollingTodos) {
			itemName = m.data.RollingTodos[cursor].Task
		}
	case 4: // Reminders
		if cursor < len(m.data.Reminders) {
			itemName = m.data.Reminders[cursor].Reminder
		}
	case 5: // Glossary
		if cursor < len(m.data.Glossary) {
			itemName = m.data.Glossary[cursor].Command
		}
	}

	if itemName != "" {
		m.confirmDelete = true
		m.deleteTarget = itemName
	}
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
	header := headerStyle.Render("📋 lif - lucas is forgetful")

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
		// Show summary stats
		totalDailies := len(m.data.Dailies)
		completedDailies := 0
		for _, daily := range m.data.Dailies {
			if daily.Status == "DONE" {
				completedDailies++
			}
		}
		summary := fmt.Sprintf("\nDaily Tasks: %d total, %d completed\n", totalDailies, completedDailies)
		summary += fmt.Sprintf("Rolling Todos: %d items\n", len(m.data.RollingTodos))
		summary += fmt.Sprintf("Reminders: %d active\n", len(m.data.Reminders))
		summary += fmt.Sprintf("Glossary: %d entries\n", len(m.data.Glossary))

		if len(m.data.RollingTodos) > 0 {
			summary += "\n" + priorityHighStyle.Render("Check your Rolling Todo List!")
		}

		// Show expired reminders
		expiredReminders := []Reminder{}
		for _, reminder := range m.data.Reminders {
			if reminder.Status == "expired" {
				expiredReminders = append(expiredReminders, reminder)
			}
		}

		if len(expiredReminders) > 0 {
			summary += "\n" + statusOverdueStyle.Render("\nExpired Reminders:") + "\n"
			for _, reminder := range expiredReminders {
				summary += fmt.Sprintf("  ⚠️  - %s\n", reminder.Reminder)
			}
		}

		// Show active reminders with countdown
		activeReminders := []Reminder{}
		for _, reminder := range m.data.Reminders {
			if !reminder.TargetTime.IsZero() && (reminder.Status == "active" || reminder.Status == "paused") {
				activeReminders = append(activeReminders, reminder)
			}
		}

		// Sort by time remaining (soonest first)
		sort.Slice(activeReminders, func(i, j int) bool {
			iRemaining := time.Until(activeReminders[i].TargetTime)
			jRemaining := time.Until(activeReminders[j].TargetTime)

			// Handle paused reminders - use PausedRemaining for comparison
			if activeReminders[i].Status == "paused" && activeReminders[i].PausedRemaining > 0 {
				iRemaining = activeReminders[i].PausedRemaining
			}
			if activeReminders[j].Status == "paused" && activeReminders[j].PausedRemaining > 0 {
				jRemaining = activeReminders[j].PausedRemaining
			}

			// Sort by remaining time (ascending - soonest first)
			return iRemaining < jRemaining
		})

		if len(activeReminders) > 0 {
			summary += "\n\n" + priorityHighStyle.Render("Active Reminders:") + "\n"
			for _, reminder := range activeReminders {
				statusIcon := "🕐"
				if reminder.Status == "paused" {
					statusIcon = "⏸️"
					// Show paused remaining time
					if reminder.PausedRemaining > 0 {
						if reminder.IsCountdown {
							summary += fmt.Sprintf("  %s %s: %s (PAUSED)\n", statusIcon, reminder.Reminder, formatDuration(reminder.PausedRemaining))
						} else {
							summary += fmt.Sprintf("  %s %s: PAUSED\n", statusIcon, reminder.Reminder)
						}
					} else {
						summary += fmt.Sprintf("  %s %s: PAUSED\n", statusIcon, reminder.Reminder)
					}
				} else {
					// Active reminder - show live countdown
					remaining := time.Until(reminder.TargetTime)
					if remaining > 0 {
						if reminder.IsCountdown {
							summary += fmt.Sprintf("  %s %s: %s\n", statusIcon, reminder.Reminder, formatDuration(remaining))
						} else {
							summary += fmt.Sprintf("  %s %s: %s\n", statusIcon, reminder.Reminder, reminder.TargetTime.Format("15:04"))
						}
					} else {
						summary += fmt.Sprintf("  ⚠️ %s: EXPIRED\n", reminder.Reminder)
					}
				}
			}
		}

		content = summary
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
		if m.activeTab == 4 {
			commands = append(commands, keyStyle.Render("s")+": "+actionStyle.Render("start/resume"))
			commands = append(commands, keyStyle.Render("p")+": "+actionStyle.Render("pause"))
			commands = append(commands, keyStyle.Render("r")+": "+actionStyle.Render("reset"))
		}
	}
	commands = append(commands, keyStyle.Render("q")+": "+actionStyle.Render("quit"))

	commandRow := strings.Join(commands, bulletStyle.Render(" • "))

	// Status message (no expiry)
	if m.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.statusColor))
		commandRow += "\n> " + statusStyle.Render(m.statusMsg)
	}

	// Delete confirmation message
	if m.confirmDelete {
		deleteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		commandRow += "\n> " + deleteStyle.Render(fmt.Sprintf("Delete '%s'? Press 'y' to confirm, 'n' to cancel", m.deleteTarget))
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
		labels = []string{"Reminder:", "Note:", "Alarm/Countdown:"}
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

	configPath := filepath.Join(configDir, "lif", "config.json")

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

	// Initialize reminders that need parsing
	for i := range data.Reminders {
		reminder := &data.Reminders[i]
		if reminder.TargetTime.IsZero() && reminder.AlarmOrCountdown != "" {
			if targetTime, isCountdown := parseCountdown(reminder.AlarmOrCountdown); isCountdown {
				reminder.TargetTime = targetTime
				reminder.IsCountdown = true
				reminder.Status = "active"
			} else if targetTime, isAlarm := parseAlarmTime(reminder.AlarmOrCountdown); isAlarm {
				reminder.TargetTime = targetTime
				reminder.IsCountdown = false
				reminder.Status = "active"
			}
		}
	}

	return data
}

func saveData(data AppData) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	configPath := filepath.Join(configDir, "lif", "config.json")

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
