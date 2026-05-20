package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	cacheTTL    = 24 * time.Hour
	githubAPI   = "https://api.github.com/repos/github/gitignore/contents/"
	rawBase     = "https://raw.githubusercontent.com/github/gitignore/main/"
	visibleRows = 15
)

// ─── Cache ────────────────────────────────────────────────────────────────────

type cacheData struct {
	FetchedAt time.Time `json:"fetched_at"`
	Templates []string  `json:"templates"`
}

func cacheFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "gitignore-cli", "templates.json")
}

func loadCache() ([]string, bool) {
	b, err := os.ReadFile(cacheFilePath())
	if err != nil {
		return nil, false
	}
	var c cacheData
	if json.Unmarshal(b, &c) != nil || time.Since(c.FetchedAt) > cacheTTL {
		return nil, false
	}
	return c.Templates, true
}

func saveCache(templates []string) {
	p := cacheFilePath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	b, _ := json.Marshal(cacheData{FetchedAt: time.Now(), Templates: templates})
	_ = os.WriteFile(p, b, 0644)
}

// ─── GitHub API ───────────────────────────────────────────────────────────────

type ghEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type (
	templatesFetchedMsg struct{ templates []string }
	contentFetchedMsg   struct {
		name    string
		content string
	}
	errMsg struct{ err error }
)

func cmdFetchTemplates() tea.Cmd {
	return func() tea.Msg {
		if t, ok := loadCache(); ok {
			return templatesFetchedMsg{t}
		}
		resp, err := http.Get(githubAPI) //nolint:noctx
		if err != nil {
			return errMsg{fmt.Errorf("fetch template list: %w", err)}
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return errMsg{fmt.Errorf("read template list: %w", err)}
		}
		var entries []ghEntry
		if err := json.Unmarshal(body, &entries); err != nil {
			return errMsg{fmt.Errorf("parse template list: %w", err)}
		}
		var names []string
		for _, e := range entries {
			if e.Type == "file" && strings.HasSuffix(e.Name, ".gitignore") {
				names = append(names, strings.TrimSuffix(e.Name, ".gitignore"))
			}
		}
		saveCache(names)
		return templatesFetchedMsg{names}
	}
}

func cmdFetchContent(name string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(rawBase + name + ".gitignore") //nolint:noctx
		if err != nil {
			return errMsg{fmt.Errorf("fetch %s template: %w", name, err)}
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return errMsg{fmt.Errorf("read %s template: %w", name, err)}
		}
		return contentFetchedMsg{name: name, content: string(body)}
	}
}

// ─── File writing ─────────────────────────────────────────────────────────────

// writeGitignore creates or appends to .gitignore in dir.
// Returns true if the content was appended to an existing non-empty file.
func writeGitignore(dir, name, content string) (appended bool, err error) {
	path := filepath.Join(dir, ".gitignore")
	existing, readErr := os.ReadFile(path)
	appended = readErr == nil && len(strings.TrimSpace(string(existing))) > 0

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if appended {
		if _, err = fmt.Fprintf(f, "\n\n# ─── %s ───\n", name); err != nil {
			return false, err
		}
	}
	_, err = f.WriteString(content)
	return appended, err
}

// ─── TUI model ────────────────────────────────────────────────────────────────

type appState int

const (
	stateLoading   appState = iota
	stateSelecting          // showing filterable list
	stateFetching           // downloading template content
	stateDone
	stateError
)

type model struct {
	textInput textinput.Model
	all       []string // all template names
	filtered  []string // filtered subset
	cursor    int      // index within filtered
	offset    int      // scroll offset
	st        appState
	targetDir string
	doneMsg   string
	err       error
}

func newModel(dir string) model {
	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40
	return model{textInput: ti, targetDir: dir, st: stateLoading}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, cmdFetchTemplates())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case templatesFetchedMsg:
		m.all = msg.templates
		m.filtered = msg.templates
		m.st = stateSelecting

	case contentFetchedMsg:
		appended, err := writeGitignore(m.targetDir, msg.name, msg.content)
		if err != nil {
			m.err = err
			m.st = stateError
		} else if appended {
			m.doneMsg = fmt.Sprintf("Appended %s template to .gitignore", msg.name)
			m.st = stateDone
		} else {
			m.doneMsg = fmt.Sprintf("Created .gitignore with %s template", msg.name)
			m.st = stateDone
		}

	case errMsg:
		m.err = msg.err
		m.st = stateError

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			switch m.st {
			case stateSelecting:
				if len(m.filtered) > 0 {
					selected := m.filtered[m.cursor]
					m.st = stateFetching
					return m, cmdFetchContent(selected)
				}
			case stateDone, stateError:
				return m, tea.Quit
			}

		case "up", "ctrl+p":
			if m.st == stateSelecting && m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset--
				}
			}

		case "down", "ctrl+n":
			if m.st == stateSelecting && m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.offset+visibleRows {
					m.offset++
				}
			}

		default:
			if m.st == stateSelecting {
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				m.filtered = filterItems(m.all, m.textInput.Value())
				if m.cursor >= len(m.filtered) {
					m.cursor = max(0, len(m.filtered)-1)
				}
				m.offset = clampOffset(m.offset, m.cursor, len(m.filtered))
				return m, cmd
			}
		}
	}

	return m, nil
}

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("255")).Bold(true).Padding(0, 1)
	normalStyle   = lipgloss.NewStyle().Padding(0, 1)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(0, 1)
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Padding(0, 1)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Padding(0, 1)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
)

func (m model) View() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("gitignore generator") + "\n\n")

	switch m.st {
	case stateLoading:
		b.WriteString(dimStyle.Render("Fetching templates from GitHub…") + "\n")

	case stateSelecting:
		b.WriteString(" " + m.textInput.View() + "\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("%d / %d templates", len(m.filtered), len(m.all))) + "\n\n")

		end := min(m.offset+visibleRows, len(m.filtered))
		for i := m.offset; i < end; i++ {
			label := fmt.Sprintf("%-32s", m.filtered[i])
			if i == m.cursor {
				b.WriteString(selectedStyle.Render("▶ "+label) + "\n")
			} else {
				b.WriteString(normalStyle.Render("  "+label) + "\n")
			}
		}

		b.WriteString("\n")
		if fileExists(filepath.Join(m.targetDir, ".gitignore")) {
			b.WriteString(dimStyle.Render("  .gitignore exists — selection will be appended") + "\n")
		} else {
			b.WriteString(dimStyle.Render("  .gitignore will be created") + "\n")
		}
		b.WriteString(helpStyle.Render("  ↑/↓  navigate   enter  select   esc  quit") + "\n")

	case stateFetching:
		b.WriteString(dimStyle.Render("Fetching template content…") + "\n")

	case stateDone:
		b.WriteString(successStyle.Render("✓ "+m.doneMsg) + "\n\n")
		b.WriteString(helpStyle.Render("Press enter to exit") + "\n")

	case stateError:
		b.WriteString(errorStyle.Render("✗ "+m.err.Error()) + "\n\n")
		b.WriteString(helpStyle.Render("Press enter to exit") + "\n")
	}

	return b.String()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func filterItems(items []string, query string) []string {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	var out []string
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), q) {
			out = append(out, item)
		}
	}
	return out
}

func clampOffset(offset, cursor, total int) int {
	if total == 0 {
		return 0
	}
	if cursor < offset {
		return cursor
	}
	if cursor >= offset+visibleRows {
		return cursor - visibleRows + 1
	}
	return offset
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error getting working directory:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(dir), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
