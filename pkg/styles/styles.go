package styles

import "github.com/charmbracelet/lipgloss"

// ── Theme Definition ──────────────────────────────────────────────────────────

type Theme struct {
	Name string

	Bg         string
	BgPanel    string
	BgHover    string
	BgSelected string

	Accent      string
	AccentDark  string
	AccentLight string

	Success    string
	SuccessDim string

	TextPrimary string
	TextSecond  string
	TextDim     string

	Border      string
	BorderFocus string
}

var (
	// Current Palette (initialized to Ocean Dark)
	ColorBg         = "#070D1A"
	ColorBgPanel    = "#0D1B2A"
	ColorBgHover    = "#122336"
	ColorBgSelected = "#1A3350"

	ColorAccent      = "#FF6B35" // Kraken Orange
	ColorAccentDark  = "#C04A14"
	ColorAccentLight = "#FF9262"

	ColorSuccess    = "#00E5A0" // Kraken Green
	ColorSuccessDim = "#1A5C47"

	ColorTextPrimary = "#E8F4FD"
	ColorTextSecond  = "#94A3B8"
	ColorTextDim     = "#4A5568"

	ColorBorder      = "#1B3A5C"
	ColorBorderFocus = "#FF6B35"

	ColorError   = "#F87171"
	ColorWarning = "#FBBF24"
)

// Themes
var (
	OceanDark = Theme{
		Name:            "Ocean (Dark)",
		Bg:              "#070D1A",
		BgPanel:         "#0D1B2A",
		BgHover:         "#122336",
		BgSelected:      "#1A3350",
		Accent:          "#FF6B35",
		AccentDark:      "#C04A14",
		AccentLight:     "#FF9262",
		Success:         "#00E5A0",
		SuccessDim:      "#1A5C47",
		TextPrimary:     "#E8F4FD",
		TextSecond:      "#94A3B8",
		TextDim:         "#4A5568",
		Border:          "#1B3A5C",
		BorderFocus:     "#FF6B35",
	}

	OceanLight = Theme{
		Name:            "Ocean (Light)",
		Bg:              "#F0F9FF",
		BgPanel:         "#E0F2FE",
		BgHover:         "#BAE6FD",
		BgSelected:      "#7DD3FC",
		Accent:          "#EA580C",
		AccentDark:      "#9A3412",
		AccentLight:     "#FB923C",
		Success:         "#059669",
		SuccessDim:      "#D1FAE5",
		TextPrimary:     "#0C4A6E",
		TextSecond:      "#075985",
		TextDim:         "#0369A1",
		Border:          "#BAE6FD",
		BorderFocus:     "#EA580C",
	}

	DraculaDark = Theme{
		Name:            "Dracula (Dark)",
		Bg:              "#282a36",
		BgPanel:         "#44475a",
		BgHover:         "#6272a4",
		BgSelected:      "#44475a",
		Accent:          "#bd93f9",
		AccentDark:      "#6272a4",
		AccentLight:     "#ff79c6",
		Success:         "#50fa7b",
		SuccessDim:      "#282a36",
		TextPrimary:     "#f8f8f2",
		TextSecond:      "#8be9fd",
		TextDim:         "#6272a4",
		Border:          "#44475a",
		BorderFocus:     "#bd93f9",
	}

	GruvboxDark = Theme{
		Name:            "Gruvbox (Dark)",
		Bg:              "#282828",
		BgPanel:         "#3c3836",
		BgHover:         "#504945",
		BgSelected:      "#665c54",
		Accent:          "#fe8019",
		AccentDark:      "#d65d0e",
		AccentLight:     "#fabd2f",
		Success:         "#b8bb26",
		SuccessDim:      "#98971a",
		TextPrimary:     "#ebdbb2",
		TextSecond:      "#a89984",
		TextDim:         "#928374",
		Border:          "#3c3836",
		BorderFocus:     "#fe8019",
	}
)

// SetTheme updates all global color variables and re-initializes styles.
func SetTheme(themeName string, darkMode bool) {
	var t Theme
	switch themeName {
	case "Ocean":
		if darkMode {
			t = OceanDark
		} else {
			t = OceanLight
		}
	case "Dracula":
		t = DraculaDark // (Simplified: Dracula usually only has dark)
	case "Gruvbox":
		t = GruvboxDark
	default:
		t = OceanDark
	}

	ColorBg = t.Bg
	ColorBgPanel = t.BgPanel
	ColorBgHover = t.BgHover
	ColorBgSelected = t.BgSelected
	ColorAccent = t.Accent
	ColorAccentDark = t.AccentDark
	ColorAccentLight = t.AccentLight
	ColorSuccess = t.Success
	ColorSuccessDim = t.SuccessDim
	ColorTextPrimary = t.TextPrimary
	ColorTextSecond = t.TextSecond
	ColorTextDim = t.TextDim
	ColorBorder = t.Border
	ColorBorderFocus = t.BorderFocus

	reinitializeStyles()
}

// ── Base Styles ───────────────────────────────────────────────────────────────

var (
	Logo                 lipgloss.Style
	Base                 lipgloss.Style
	Dim                  lipgloss.Style
	Muted                lipgloss.Style
	Bold                 lipgloss.Style
	AppTitle             lipgloss.Style
	ChatUserMsg          lipgloss.Style
	ChatUserBubble       lipgloss.Style
	ChatAIMsg            lipgloss.Style
	ChatAIBubble         lipgloss.Style
	ChatSystemMsg        lipgloss.Style
	ChatSessionTab       lipgloss.Style
	ChatSessionTabActive lipgloss.Style
	ChatSpinner          lipgloss.Style
	ChatInput            lipgloss.Style
	FileDir              lipgloss.Style
	FileRegular          lipgloss.Style
	FileHidden           lipgloss.Style
	FileExec             lipgloss.Style
	FileSelected         lipgloss.Style
	FileSize             lipgloss.Style
	FilePerm             lipgloss.Style
	FilePath             lipgloss.Style
	FilePrompt           lipgloss.Style
	FileConfirmDanger    lipgloss.Style
	TodoDone             lipgloss.Style
	TodoPending          lipgloss.Style
	TodoSelected         lipgloss.Style
	TodoCheckDone        lipgloss.Style
	TodoCheckPending     lipgloss.Style
	TodoInput            lipgloss.Style
	TodoCount            lipgloss.Style
	StatusBar            lipgloss.Style
	StatusKey            lipgloss.Style
	StatusVal            lipgloss.Style
	StatusErr            lipgloss.Style
	StatusOk             lipgloss.Style
)

func init() {
	reinitializeStyles()
}

func reinitializeStyles() {
	Logo = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).MarginLeft(1)
	Base = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextPrimary))
	Dim = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextDim))
	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextDim)) // Simplified
	Bold = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextPrimary)).Bold(true)
	AppTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true)

	// File Browser
	FileDir = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccentLight)).Bold(true)
	FileRegular = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextPrimary))
	FileHidden = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextDim))
	FileExec = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	FileSelected = lipgloss.NewStyle().Background(lipgloss.Color(ColorBgSelected)).Foreground(lipgloss.Color(ColorAccentLight)).Bold(true)
	FileSize = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextSecond)).Width(8).Align(lipgloss.Right)
	FilePerm = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextDim)).Width(10)
	FilePath = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextSecond)).Italic(true)
	FilePrompt = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	FileConfirmDanger = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true)

	// Chat
	ChatUserMsg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	ChatUserBubble = lipgloss.NewStyle().Background(lipgloss.Color(ColorAccentDark)).Foreground(lipgloss.Color(ColorTextPrimary)).Padding(0, 1).MarginBottom(1)
	ChatAIMsg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true)
	ChatAIBubble = lipgloss.NewStyle().Background(lipgloss.Color(ColorSuccessDim)).Foreground(lipgloss.Color(ColorTextPrimary)).Padding(0, 1).MarginBottom(1)
	ChatSystemMsg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextDim)).Italic(true)
	ChatSessionTab = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color(ColorTextDim))
	ChatSessionTabActive = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color(ColorSuccess)).Bold(true).Underline(true)
	ChatSpinner = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	ChatInput = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(lipgloss.Color(ColorBorder)).Padding(0, 1)

	// Todo
	TodoDone = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccessDim)).Strikethrough(true)
	TodoPending = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextPrimary))
	TodoSelected = lipgloss.NewStyle().Background(lipgloss.Color(ColorBgSelected)).Foreground(lipgloss.Color(ColorAccentLight)).Bold(true)
	TodoCheckDone = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true)
	TodoCheckPending = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextDim))
	TodoInput = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent))
	TodoCount = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextSecond)).Italic(true)

	// Status
	StatusBar = lipgloss.NewStyle().Background(lipgloss.Color(ColorBorder)).Foreground(lipgloss.Color(ColorTextPrimary)).Padding(0, 1)
	StatusKey = lipgloss.NewStyle().Background(lipgloss.Color(ColorAccentDark)).Foreground(lipgloss.Color(ColorTextPrimary)).Padding(0, 1).Bold(true)
	StatusVal = lipgloss.NewStyle().Background(lipgloss.Color(ColorBorder)).Foreground(lipgloss.Color(ColorTextSecond)).Padding(0, 1)
	StatusErr = lipgloss.NewStyle().Background(lipgloss.Color(ColorBg)).Foreground(lipgloss.Color(ColorError)).Padding(0, 1).Bold(true)
	StatusOk = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Padding(0, 1)
}

// ── Panel Borders ─────────────────────────────────────────────────────────────

var commonBorder = lipgloss.Border{
	Top:         "─",
	Bottom:      "─",
	Left:        "│",
	Right:       "│",
	TopLeft:     "╭",
	TopRight:    "╮",
	BottomLeft:  "╰",
	BottomRight: "╯",
}

func PanelInactive(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().Border(commonBorder).BorderForeground(lipgloss.Color(ColorBorder)).Foreground(lipgloss.Color(ColorTextPrimary)).Width(width).Height(height).Padding(0, 1)
}

func PanelActive(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().Border(commonBorder).BorderForeground(lipgloss.Color(ColorBorderFocus)).Foreground(lipgloss.Color(ColorTextPrimary)).Width(width).Height(height).Padding(0, 1)
}

func PanelTitle(active bool) lipgloss.Style {
	c := ColorTextSecond
	if active {
		c = ColorAccent
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Bold(true).Padding(0, 1)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func HelpPill(key, desc string) string {
	k := StatusKey.Render(key)
	v := StatusVal.Render(desc)
	return k + v
}
