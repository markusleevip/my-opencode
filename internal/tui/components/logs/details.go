package logs

import (
	"fmt"
	"strings"
	"time"

	"myopencode/internal/logging"
	"myopencode/internal/tui/layout"
	"myopencode/internal/tui/styles"
	"myopencode/internal/tui/theme"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DetailComponent interface {
	tea.Model
	layout.Sizeable
	layout.Bindings
}

type detailCmp struct {
	width, height int
	currentLog    logging.LogMessage
	viewport      viewport.Model
}

func (i *detailCmp) Init() tea.Cmd {
	messages := logging.List()
	if len(messages) == 0 {
		return nil
	}
	i.currentLog = messages[0]
	return nil
}

func (i *detailCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case selectedLogMsg:
		if msg.ID != i.currentLog.ID {
			i.currentLog = logging.LogMessage(msg)
			i.updateContent()
		}
	}

	return i, nil
}

func (i *detailCmp) updateContent() {
	var content strings.Builder
	t := theme.CurrentTheme()

	if i.width <= 0 {
		return
	}

	// Format the header with timestamp and level
	timeStyle := lipgloss.NewStyle().Foreground(t.TextMuted())
	levelStyle := getLevelStyle(i.currentLog.Level)

	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		timeStyle.Render(i.currentLog.Time.Format(time.RFC3339)),
		"  ",
		levelStyle.Render(i.currentLog.Level),
	)

	content.WriteString(lipgloss.NewStyle().Bold(true).Render(header))
	content.WriteString("\n\n")

	// Message with styling and wrapping
	messageStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Primary())
	content.WriteString(messageStyle.Render("Message:"))
	content.WriteString("\n")

	wordWrapStyle := lipgloss.NewStyle().Padding(0, 2).Width(i.width - 4)
	content.WriteString(wordWrapStyle.Render(i.currentLog.Message))
	content.WriteString("\n\n")

	// Attributes section
	if len(i.currentLog.Attributes) > 0 {
		attrHeaderStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Primary())
		content.WriteString(attrHeaderStyle.Render("Attributes:"))
		content.WriteString("\n")

		// Create a wrapped display for attributes
		keyStyle := lipgloss.NewStyle().Foreground(t.Primary()).Bold(true)
		valueStyle := lipgloss.NewStyle().Foreground(t.Text())

		for _, attr := range i.currentLog.Attributes {
			attrLine := fmt.Sprintf("%s: %s",
				keyStyle.Render(attr.Key),
				valueStyle.Render(attr.Value),
			)
			content.WriteString(lipgloss.NewStyle().Padding(0, 2).Width(i.width - 4).Render(attrLine))
			content.WriteString("\n")
		}
	}

	i.viewport.SetContent(content.String())
}

func getLevelStyle(level string) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true)
	t := theme.CurrentTheme()

	switch strings.ToLower(level) {
	case "info":
		return style.Foreground(t.Info())
	case "warn", "warning":
		return style.Foreground(t.Warning())
	case "error", "err":
		return style.Foreground(t.Error())
	case "debug":
		return style.Foreground(t.Success())
	default:
		return style.Foreground(t.Text())
	}
}

func (i *detailCmp) View() string {
	t := theme.CurrentTheme()
	return styles.ForceReplaceBackgroundWithLipgloss(i.viewport.View(), t.Background())
}

func (i *detailCmp) GetSize() (int, int) {
	return i.width, i.height
}

func (i *detailCmp) SetSize(width int, height int) tea.Cmd {
	i.width = width
	i.height = height
	i.viewport.Width = i.width
	i.viewport.Height = i.height
	i.updateContent()
	return nil
}

func (i *detailCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(i.viewport.KeyMap)
}

func NewLogsDetails() DetailComponent {
	return &detailCmp{
		viewport: viewport.New(0, 0),
	}
}
