package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/musistudio/ccg/internal/config"
)

// ViewMode represents which panel is focused
type ViewMode int

const (
	ViewProviders ViewMode = iota
	ViewRouter
	ViewTransformers
)

// Model represents the main TUI application
type Model struct {
	config   *config.Config
	viewMode ViewMode
	width    int
	height   int

	// Providers state
	selectedProvider  int
	editingProvider   bool
	providerScrollOff int

	// Router state
	selectedRouterField int
	routerDropdownOpen  bool
	routerDropdownField int

	// Transformers state
	selectedTransformer int

	// Dialog state
	dialogOpen bool
	dialogType string

	// Status
	statusMsg     string
	serverRunning bool
	requestCount  int
	tokenCount    int
}

// New creates a new TUI model
func New() Model {
	return Model{
		config:        config.New(),
		viewMode:      ViewProviders,
		serverRunning: true,
		requestCount:  0,
		tokenCount:    0,
	}
}

// Init initializes the TUI model
func (m Model) Init() tea.Cmd {
	return tea.WindowSize()
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		m.handleKeyboard(msg)

	case tea.MouseMsg:
		m.handleMouse(msg)
	}

	return m, nil
}

func (m *Model) handleKeyboard(msg tea.KeyMsg) {
	// Handle dropdown open state
	if m.routerDropdownOpen {
		switch msg.String() {
		case "esc", "q":
			m.routerDropdownOpen = false
		case "up", "k":
			if m.selectedRouterField > 0 {
				m.selectedRouterField--
			}
		case "down", "j":
			if m.selectedRouterField < 5 {
				m.selectedRouterField++
			}
		case "enter":
			m.routerDropdownOpen = false
			m.statusMsg = fmt.Sprintf("Selected router field %d", m.selectedRouterField+1)
		}
		return
	}

	// Handle dialog open state
	if m.dialogOpen {
		switch msg.String() {
		case "esc":
			m.dialogOpen = false
		}
		return
	}

	// Normal keyboard handling
	switch msg.String() {
	case "ctrl+c":
		// Quit handled by Run()
	case "q":
		// Only quit if no dialog/dropdown open
		if !m.dialogOpen && !m.routerDropdownOpen {
			// Quit handled by Run()
		}
	case "tab":
		m.viewMode = (m.viewMode + 1) % 3
	case "1":
		m.viewMode = ViewProviders
	case "2":
		m.viewMode = ViewRouter
	case "3":
		m.viewMode = ViewTransformers
	case "s":
		m.saveConfig()
	case "j", "down":
		m.handleDown()
	case "k", "up":
		m.handleUp()
	case "n":
		if m.viewMode == ViewProviders {
			m.addProvider()
		} else if m.viewMode == ViewTransformers {
			m.addTransformer()
		}
	case "d":
		if m.viewMode == ViewProviders {
			m.deleteProvider()
		}
	case "enter":
		m.handleEnter()
	case "r":
		// Refresh
		m.statusMsg = "Refreshed"
	}
}

func (m *Model) handleMouse(msg tea.MouseMsg) {
	switch msg.Type {
	case tea.MouseWheelUp:
		m.handleUp()
	case tea.MouseWheelDown:
		m.handleDown()
	case tea.MouseLeft:
		m.handleClick(msg.X, msg.Y)
	}
}

func (m *Model) handleClick(x, y int) {
	// Calculate layout dimensions
	headerHeight := 2
	contentHeight := m.height - headerHeight - 2 // -2 for status bar
	leftWidth := m.width * 3 / 5
	_ = m.width - leftWidth // rightWidth (for future use)
	routerHeight := contentHeight * 3 / 5

	// Header row (y = 0 or 1)
	if y <= 1 {
		// Calculate button positions
		titleEnd := 5
		providersTabStart := titleEnd + 2
		providersTabEnd := providersTabStart + 12
		routerTabStart := providersTabEnd + 1
		routerTabEnd := routerTabStart + 9
		transformersTabStart := routerTabEnd + 1
		transformersTabEnd := transformersTabStart + 15

		// Right side buttons
		saveBtnStart := m.width - 8
		presetsBtnStart := saveBtnStart - 10
		logsBtnStart := presetsBtnStart - 7
		settingsBtnStart := logsBtnStart - 10

		if x >= providersTabStart && x <= providersTabEnd {
			m.viewMode = ViewProviders
		} else if x >= routerTabStart && x <= routerTabEnd {
			m.viewMode = ViewRouter
		} else if x >= transformersTabStart && x <= transformersTabEnd {
			m.viewMode = ViewTransformers
		} else if x >= saveBtnStart {
			m.saveConfig()
		} else if x >= presetsBtnStart && x < saveBtnStart {
			m.statusMsg = "Presets opened"
		} else if x >= logsBtnStart && x < presetsBtnStart {
			m.statusMsg = "Logs opened"
		} else if x >= settingsBtnStart && x < logsBtnStart {
			m.statusMsg = "Settings opened"
		}
		return
	}

	// Content area
	contentY := y - headerHeight

	// Check if click is in left panel (Providers)
	if x < leftWidth && contentY >= 0 && contentY < contentHeight {
		m.viewMode = ViewProviders

		// [+ Add] button is at line 2
		if contentY == 2 && x >= 20 && x <= 30 {
			m.addProvider()
			return
		}

		// Provider items start at line 4, each provider is 4 lines
		providerStartLine := 4
		for i := 0; i < 10; i++ { // Check up to 10 providers
			providerTop := providerStartLine + i*4
			providerBottom := providerTop + 3
			if contentY >= providerTop && contentY <= providerBottom {
				m.selectedProvider = i
				m.editingProvider = true
				m.dialogOpen = true
				m.dialogType = "provider"
				return
			}
		}
	}

	// Check if click is in right top panel (Router)
	rightStartX := leftWidth + 1
	if x >= rightStartX && contentY >= 0 && contentY < routerHeight {
		m.viewMode = ViewRouter

		// Router fields start at line 2, each field is 3 lines
		fieldStartLine := 2
		for i := 0; i < 5; i++ {
			fieldTop := fieldStartLine + i*3
			fieldBottom := fieldTop + 2
			if contentY >= fieldTop && contentY <= fieldBottom {
				m.selectedRouterField = i
				m.routerDropdownOpen = true
				m.routerDropdownField = i
				return
			}
		}
	}

	// Check if click is in right bottom panel (Transformers)
	transformerStartY := routerHeight
	if x >= rightStartX && contentY >= transformerStartY {
		m.viewMode = ViewTransformers

		// [+ Add] button is 2 lines after transformer start
		if contentY == transformerStartY+2 && x >= rightStartX+2 && x <= rightStartX+12 {
			m.addTransformer()
			return
		}

		// Transformer items start at line 4 after transformer start
		itemStartLine := transformerStartY + 4
		for i := 0; i < 5; i++ {
			itemLine := itemStartLine + i
			if contentY == itemLine {
				m.selectedTransformer = i
				return
			}
		}
	}
}

func (m *Model) handleDown() {
	switch m.viewMode {
	case ViewProviders:
		providers := m.config.GetProviders()
		if m.selectedProvider < len(providers)-1 {
			m.selectedProvider++
		}
	case ViewRouter:
		if m.selectedRouterField < 4 {
			m.selectedRouterField++
		}
	case ViewTransformers:
		if m.selectedTransformer < 2 {
			m.selectedTransformer++
		}
	}
}

func (m *Model) handleUp() {
	switch m.viewMode {
	case ViewProviders:
		if m.selectedProvider > 0 {
			m.selectedProvider--
		}
	case ViewRouter:
		if m.selectedRouterField > 0 {
			m.selectedRouterField--
		}
	case ViewTransformers:
		if m.selectedTransformer > 0 {
			m.selectedTransformer--
		}
	}
}

func (m *Model) handleEnter() {
	switch m.viewMode {
	case ViewProviders:
		m.editingProvider = true
		m.dialogOpen = true
		m.dialogType = "provider"
	case ViewRouter:
		m.routerDropdownOpen = true
		m.routerDropdownField = m.selectedRouterField
	}
}

func (m *Model) addProvider() {
	m.statusMsg = "Add new provider"
	m.editingProvider = true
	m.dialogOpen = true
	m.dialogType = "provider"
	m.selectedProvider = len(m.config.GetProviders())
}

func (m *Model) deleteProvider() {
	providers := m.config.GetProviders()
	if len(providers) > 0 && m.selectedProvider < len(providers) {
		m.statusMsg = fmt.Sprintf("Deleted: %s", providers[m.selectedProvider].Name)
	}
}

func (m *Model) addTransformer() {
	m.statusMsg = "Add new transformer"
}

func (m *Model) saveConfig() {
	m.statusMsg = "Config saved!"
}

// View renders the TUI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Load config
	configPath := config.GetDefaultConfigPath()
	m.config.Load(configPath)

	// Calculate layout
	headerHeight := 2
	contentHeight := m.height - headerHeight - 2
	leftWidth := m.width * 3 / 5
	rightWidth := m.width - leftWidth - 1
	routerHeight := contentHeight * 3 / 5
	transformerHeight := contentHeight - routerHeight

	var sb strings.Builder

	// Header
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")

	// Content panels side by side
	leftPanel := m.renderProviders(leftWidth, contentHeight)
	rightTopPanel := m.renderRouter(rightWidth, routerHeight)
	rightBottomPanel := m.renderTransformers(rightWidth, transformerHeight-1)

	// Combine panels
	leftLines := strings.Split(leftPanel, "\n")
	rightTopLines := strings.Split(rightTopPanel, "\n")
	rightBottomLines := strings.Split(rightBottomPanel, "\n")
	rightLines := append(rightTopLines, rightBottomLines...)

	// Render rows
	maxLines := contentHeight
	for i := 0; i < maxLines; i++ {
		// Left panel line
		leftLine := ""
		if i < len(leftLines) {
			leftLine = leftLines[i]
		}
		leftPadded := lipgloss.NewStyle().Width(leftWidth).Render(leftLine)

		// Right panel line
		rightLine := ""
		if i < len(rightLines) {
			rightLine = rightLines[i]
		}
		rightPadded := lipgloss.NewStyle().Width(rightWidth).Render(rightLine)

		sb.WriteString(leftPadded + "│" + rightPadded + "\n")
	}

	// Status bar
	sb.WriteString(m.renderStatusBar())

	return sb.String()
}

func (m Model) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	tabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Padding(0, 1)

	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	btnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Padding(0, 1)

	// Title
	title := titleStyle.Render("CCG")

	// Tabs
	providersTab := tabStyle.Render("Providers")
	routerTab := tabStyle.Render("Router")
	transformersTab := tabStyle.Render("Transformers")

	switch m.viewMode {
	case ViewProviders:
		providersTab = activeTabStyle.Render("Providers")
	case ViewRouter:
		routerTab = activeTabStyle.Render("Router")
	case ViewTransformers:
		transformersTab = activeTabStyle.Render("Transformers")
	}

	// Right buttons
	settingsBtn := btnStyle.Render("Settings")
	logsBtn := btnStyle.Render("Logs")
	presetsBtn := btnStyle.Render("Presets")
	saveBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#04B575")).
		Padding(0, 1).
		Render("Save")

	// Calculate spacing
	leftPart := title + " " + providersTab + " " + routerTab + " " + transformersTab
	rightPart := settingsBtn + " " + logsBtn + " " + presetsBtn + " " + saveBtn
	leftLen := lipgloss.Width(leftPart)
	rightLen := lipgloss.Width(rightPart)
	space := m.width - leftLen - rightLen
	if space < 1 {
		space = 1
	}

	header := leftPart + strings.Repeat(" ", space) + rightPart
	return header
}

func (m Model) renderProviders(width, height int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	searchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262"))

	addBtnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575"))

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444")).
		Width(width-4).
		Padding(0, 1)

	selectedCardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Background(lipgloss.Color("#2D2D2D")).
		Width(width-4).
		Padding(0, 1)

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render("Providers") + "\n")
	sb.WriteString(strings.Repeat("─", width-2) + "\n")

	// Search bar and Add button
	searchBar := searchStyle.Render("Search...") + strings.Repeat(" ", width-20) + addBtnStyle.Render("[+ Add]")
	sb.WriteString(searchBar + "\n\n")

	// Provider list
	providers := m.config.GetProviders()
	if len(providers) == 0 {
		sb.WriteString(searchStyle.Render("No providers configured"))
	} else {
		for i, p := range providers {
			modelsCount := len(p.Models)
			card := fmt.Sprintf("%s\n%s\n%d models", p.Name, p.Host, modelsCount)

			if i == m.selectedProvider && m.viewMode == ViewProviders {
				sb.WriteString(selectedCardStyle.Render(card) + "\n")
			} else {
				sb.WriteString(cardStyle.Render(card) + "\n")
			}
		}
	}

	return sb.String()
}

func (m Model) renderRouter(width, height int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4"))

	dropdownArrow := " ▼"

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render("Router") + "\n")
	sb.WriteString(strings.Repeat("─", width-2) + "\n")

	// Get router config
	router := m.config.GetRouter()
	fields := []struct {
		name  string
		value string
	}{
		{"Default", ""},
		{"Background", ""},
		{"Think", ""},
		{"Long Context", ""},
		{"Web Search", ""},
	}

	if router != nil {
		fields[0].value = router.Default
		fields[1].value = router.Background
		fields[2].value = router.Think
		fields[3].value = router.LongContext
		fields[4].value = router.WebSearch
	}

	for i, field := range fields {
		value := field.value
		if value == "" {
			value = "Select model..."
		}

		label := labelStyle.Render(field.name + ":")
		dropdown := valueStyle.Render(value) + dropdownArrow

		line := label + " " + dropdown

		if i == m.selectedRouterField && m.viewMode == ViewRouter {
			sb.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}

		// Add threshold field after Long Context
		if i == 3 {
			thresholdLabel := labelStyle.Render("Threshold:")
			thresholdValue := "60000"
			if router != nil {
				thresholdValue = fmt.Sprintf("%d", router.LongContextThreshold)
			}
			sb.WriteString("  " + thresholdLabel + " " + valueStyle.Render(thresholdValue) + "\n")
		}
	}

	// Dropdown overlay
	if m.routerDropdownOpen && m.viewMode == ViewRouter {
		sb.WriteString("\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#1E1E1E")).
			Render("Select a model from Providers...") + "\n")
	}

	return sb.String()
}

func (m Model) renderTransformers(width, height int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	addBtnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575"))

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FAFAFA"))

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render("Transformers") + "\n")
	sb.WriteString(strings.Repeat("─", width-2) + "\n")

	// Add button
	sb.WriteString(addBtnStyle.Render("[+ Add]") + "\n\n")

	// Transformer list
	transformers := []string{"maxtoken", "reasoning", "enhancetool"}
	for i, t := range transformers {
		line := "• " + t
		if i == m.selectedTransformer && m.viewMode == ViewTransformers {
			sb.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}

	return sb.String()
}

func (m Model) renderStatusBar() string {
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262"))

	statusIndicator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Render("● Running")

	status := fmt.Sprintf("%s | Requests: %d | Tokens: %d",
		statusIndicator, m.requestCount, m.tokenCount)

	help := "1-3: Switch | Tab: Next | ↑↓: Navigate | Enter: Select | n: New | s: Save | q: Quit"

	if m.statusMsg != "" {
		msgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575"))
		status = msgStyle.Render(m.statusMsg) + " | " + status
	}

	space := m.width - lipgloss.Width(status) - lipgloss.Width(help) - 2
	if space < 1 {
		space = 1
	}

	return statusStyle.Render(status + strings.Repeat(" ", space) + help)
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := p.Run()
	return err
}
