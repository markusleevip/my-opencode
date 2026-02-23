package logs

import (
	"encoding/json"
	"slices"

	"myopencode/internal/logging"
	"myopencode/internal/pubsub"
	"myopencode/internal/tui/layout"
	"myopencode/internal/tui/styles"
	"myopencode/internal/tui/theme"
	"myopencode/internal/tui/util"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type TableComponent interface {
	tea.Model
	layout.Sizeable
	layout.Bindings
}

type tableCmp struct {
	table table.Model
}

type selectedLogMsg logging.LogMessage

func (i *tableCmp) Init() tea.Cmd {
	i.setRows()
	return nil
}

func (i *tableCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg.(type) {
	case pubsub.Event[logging.LogMessage]:
		i.setRows()
		return i, nil
	}
	t, cmd := i.table.Update(msg)
	cmds = append(cmds, cmd)
	i.table = t
	selectedRow := i.table.SelectedRow()
	if selectedRow != nil {
		logs := getSortedLogs()
		cursor := i.table.Cursor()
		if cursor >= 0 && cursor < len(logs) {
			cmds = append(cmds, util.CmdHandler(selectedLogMsg(logs[cursor])))
		}
	}
	return i, tea.Batch(cmds...)
}

func (i *tableCmp) View() string {
	t := theme.CurrentTheme()
	defaultStyles := table.DefaultStyles()
	defaultStyles.Selected = defaultStyles.Selected.Foreground(t.Primary())
	i.table.SetStyles(defaultStyles)
	return styles.ForceReplaceBackgroundWithLipgloss(i.table.View(), t.Background())
}

func (i *tableCmp) GetSize() (int, int) {
	return i.table.Width(), i.table.Height()
}

func (i *tableCmp) SetSize(width int, height int) tea.Cmd {
	i.table.SetWidth(width)
	i.table.SetHeight(height)
	columns := i.table.Columns()
	// Distribute width: Time (10%), Message (60%), Attributes (30%)
	if len(columns) >= 3 {
		columns[0].Width = (width * 10 / 100) - 2 // Time
		columns[1].Width = (width * 60 / 100) - 2 // Message
		columns[2].Width = (width * 30 / 100) - 2 // Attributes
	}
	i.table.SetColumns(columns)
	return nil
}

func getSortedLogs() []logging.LogMessage {
	logs := logging.List()
	slices.SortFunc(logs, func(a, b logging.LogMessage) int {
		if a.Time.Before(b.Time) {
			return 1
		}
		if a.Time.After(b.Time) {
			return -1
		}
		return 0
	})
	return logs
}

func (i *tableCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(i.table.KeyMap)
}

func (i *tableCmp) setRows() {
	rows := []table.Row{}
	logs := getSortedLogs()

	for _, log := range logs {
		bm, _ := json.Marshal(log.Attributes)

		row := table.Row{
			log.Time.Format("15:04:05"),
			log.Message,
			string(bm),
		}
		rows = append(rows, row)
	}
	i.table.SetRows(rows)
}

func NewLogsTable() TableComponent {
	columns := []table.Column{
		{Title: "Time", Width: 10},
		{Title: "Message", Width: 50},
		{Title: "Attributes", Width: 30},
	}

	tableModel := table.New(
		table.WithColumns(columns),
	)
	tableModel.Focus()
	return &tableCmp{
		table: tableModel,
	}
}
