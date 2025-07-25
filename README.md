# lif - Lucas is Forgetful

A terminal user interface (TUI) application for managing daily tasks, reminders, rolling todos, and a command glossary. Perfect for those moments when you need a digital second brain to keep track of everything.

## Features

### 📋 Daily Tasks
- Create recurring daily tasks that reset at 3 AM
- Track completion status with visual indicators
- Organize by priority (HIGH/MEDIUM/LOW) and category
- Set deadlines and monitor progress

### 🔄 Rolling Todos
- Persistent todo items that don't reset daily
- Priority-based organization
- Category grouping for better organization
- Deadline tracking

### ⏰ Reminders & Alarms
- Set countdown timers (1m, 30s, 2h, 5d, etc.)
- Schedule alarms for specific times (9:30AM, 15:30)
- Pause and resume countdowns
- System notifications with sound alerts
- Cross-platform notification support (Linux, macOS, Windows, WSL)

### 📚 Command Glossary
- Store frequently used commands and their meanings
- Organize by programming language or category
- Quick reference with usage examples
- Perfect for remembering complex CLI commands

## Installation

### Prerequisites
- Go 1.23.3 or later

### Build from Source
```bash
git clone <repository-url>
cd lif
go build -o lif main.go
```

### Run
```bash
./lif
```

## Usage

### Navigation
- **Numbers 1-5**: Switch between tabs
- **Left/Right arrows**: Navigate tabs
- **Up/Down arrows** or **j/k**: Navigate within tables

### Basic Operations
- **e**: Edit selected item
- **n** or **a**: Add new item
- **d**: Delete selected item (with confirmation)
- **q**: Quit application

### Tab-Specific Controls

#### Daily Tasks (Tab 2)
- **Space** or **Enter**: Toggle task completion
- Tasks automatically reset to incomplete at 3 AM daily

#### Reminders (Tab 4)
- **s**: Start/resume reminder
- **p**: Pause active reminder
- **r**: Reset reminder to original time

### Time Formats

#### For Countdowns
- **Seconds**: `30s`, `45sec`
- **Minutes**: `5m`, `30min`
- **Hours**: `2h`, `3hr`
- **Days**: `1d`, `7d`
- **Weeks**: `1w`, `2w`

#### For Alarms
- **12-hour format**: `9:30AM`, `2:15 PM`
- **24-hour format**: `09:30`, `14:15`

## Configuration

Configuration is automatically saved to:
-  `~/.config/lif/config.json`

## Features in Detail

### Smart Notifications
- Audio alerts with fallback to system beep
- Supports multiple audio formats (MP3, WAV)
- WSL-compatible notification system

### Priority System
- **HIGH**: Red styling, highest priority in sorting
- **MEDIUM**: Yellow styling, default priority
- **LOW**: Green styling, lowest priority

### Data Persistence
- All data automatically saved to JSON configuration
- No external database required
- Portable configuration file

### Visual Design
- Color-coded priority indicators
- Modern table styling with clean borders
- Status-aware color themes
- Responsive layout that adapts to terminal size

## Keyboard Shortcuts Reference

| Key | Action | Context |
|-----|--------|---------|
| `1-5` | Switch tabs | Global |
| `←/→` | Navigate tabs | Global |
| `↑/↓` or `j/k` | Navigate items | Tables |
| `e` | Edit selected | Tables |
| `n/a` | Add new item | Tables |
| `d` | Delete item | Tables |
| `Space/Enter` | Toggle completion | Daily Tasks |
| `s` | Start/resume | Reminders |
| `p` | Pause | Reminders |
| `r` | Reset | Reminders |
| `q` | Quit | Global |

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling