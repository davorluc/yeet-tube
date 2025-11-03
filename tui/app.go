package tui

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
	"yeet-tube/downloader"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Each video in the queue
type VideoDownload struct {
	URL          string
	Name         string
	Percent      float64
	Log          []string
	ProgressCh   chan downloader.ProgressFractionMsg
	Done         bool
	TitleFetched bool
}

// Top-level TUI model
type model struct {
	textInput      textinput.Model
	status         string
	videoQueue     []*VideoDownload
	history        []downloader.VideoInfo
	selectedIndex  int
	windowWidth    int
	windowHeight   int
	hexStream      string // random hex string for bottom-right box
	downloadFormat string // "mp4" or "mp3"
}

// Messages
type tickMsg struct{}
type titleFetchedMsg struct {
	url   string
	title string
	err   error
}
type setFormatMsg struct {
	format string
}

// Initialize model
func InitialModel() model {
	ti := textinput.New()
	ti.Placeholder = "ENTER TEMPORAL SEQUENCE CODE..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	rand.Seed(time.Now().UnixNano())

	return model{
		textInput:      ti,
		status:         "SYSTEM ONLINE • READY FOR VARIANT INGEST",
		videoQueue:     []*VideoDownload{},
		history:        loadHistory("downloads.json"),
		selectedIndex:  0,
		windowWidth:    120,
		windowHeight:   40,
		hexStream:      randomHexString(16),
		downloadFormat: "mp4", // Default to mp4
	}
}

// Init
func (m model) Init() tea.Cmd {
	return tickCmd()
}

// Update
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

	case titleFetchedMsg:
		for _, vd := range m.videoQueue {
			if vd.URL == msg.url {
				if msg.err == nil && msg.title != "" {
					vd.Name = truncateString(strings.ToUpper(msg.title), 28)
					vd.TitleFetched = true
					m.status = fmt.Sprintf("✔ VARIANT CASE IDENTIFIED: %s", vd.Name)
				} else {
					vd.Name = truncateString(strings.ToUpper(msg.url), 28)
					vd.TitleFetched = true
					if msg.err != nil {
						m.status = "⚠ CASE IDENTIFICATION FAILED • USING RAW SEQUENCE"
					}
				}
				break
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "m":
			if m.downloadFormat == "mp4" {
				m.downloadFormat = "mp3"
			} else {
				m.downloadFormat = "mp4"
			}
		case "enter":
			url := strings.TrimSpace(m.textInput.Value())
			if url == "" {
				m.status = "⚠ INPUT REJECTED • INVALID VARIANT SEQUENCE"
				break
			}

			vd := &VideoDownload{
				URL:          url,
				Name:         "◉ SCANNING TIMELINE...",
				Percent:      0,
				Log:          []string{},
				ProgressCh:   make(chan downloader.ProgressFractionMsg, 50),
				Done:         false,
				TitleFetched: false,
			}

			m.videoQueue = append(m.videoQueue, vd)
			m.status = "✔ VARIANT SEQUENCE ACCEPTED • INITIATING CASE ANALYSIS"
			m.textInput.SetValue("")

			cmds = append(cmds, fetchTitleCmd(vd.URL))
			cmds = append(cmds, startDownloadCmd(vd, m.downloadFormat))
		case "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "down":
			if m.selectedIndex < len(m.history)-1 {
				m.selectedIndex++
			}
		}

	case setFormatMsg:
		m.downloadFormat = msg.format
		m.hexStream = formattedHexStream(7, 6)

		for _, vd := range m.videoQueue {
			if vd.Done {
				continue
			}
			select {
			case progressMsg, ok := <-vd.ProgressCh:
				if !ok {
					vd.Done = true
					m.status = fmt.Sprintf("✔ ARCHIVE COMPLETE • %s", vd.Name)

					// reload history so new file appears in list
					m.history = loadHistory("downloads.json")
					if m.selectedIndex >= len(m.history) {
						m.selectedIndex = len(m.history) - 1
					}
					break
				}

				if progressMsg.Fraction >= 0 {
					vd.Percent = progressMsg.Fraction
				}

				if progressMsg.Line != "" {
					vd.Log = append(vd.Log, progressMsg.Line)
					if len(vd.Log) > 5 {
						vd.Log = vd.Log[1:]
					}
				}

				if progressMsg.Fraction > 0 && progressMsg.Fraction < 1 && vd.TitleFetched {
					m.status = fmt.Sprintf("◉ ARCHIVING VARIANT: %s [%.1f%%]", vd.Name, progressMsg.Fraction*100)
				}

			default:
			}
		}
		cmds = append(cmds, tickCmd())
	}

	m.textInput, _ = m.textInput.Update(msg)
	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m model) View() string {
	leftWidth := int(float64(m.windowWidth) * 0.35)
	rightWidth := m.windowWidth - leftWidth - 8
	topHeight := m.windowHeight - 15
	bottomLeft := int(float64(m.windowWidth) * 0.85)

	if leftWidth < 20 {
		leftWidth = 20
	}
	if rightWidth < 40 {
		rightWidth = 40
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color("#F9BE5E")).
		Foreground(lipgloss.Color("#1A1A1A")).
		Padding(0, 1).
		Width(m.windowWidth).
		Align(lipgloss.Center)

	queueBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#F9BE5E")).
		Padding(1).
		Width(leftWidth).
		Height(topHeight).
		MarginLeft(2)

	previewBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#F9BE5E")).
		Padding(1).
		Width(rightWidth).
		Height(topHeight / 2)

	timelineBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#F9BE5E")).
		Padding(1).
		Width(rightWidth).
		Height(topHeight / 2)

	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#F9BE5E")).
		Padding(1).
		Width(bottomLeft).
		MarginLeft(2)

	hexBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#F9BE5E")).
		Foreground(lipgloss.Color("#FFcc99")).
		Width(m.windowWidth - bottomLeft - 9).
		Height(m.windowHeight - topHeight - 8).
		MarginLeft(1)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9BE5E")).
		MarginLeft(2).
		Bold(true)

	// Header
	header := headerStyle.Render("TIME VARIANCE AUTHORITY - YEET-TUBE ARCHIVAL CONSOLE v0.2.0")

	// Queue/history box
	queueTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F9BE5E")).
		Render("ARCHIVE HISTORY & ACTIVE CASES")

	queueContent := queueTitle + "\n\n"

	// Active downloads (progress bars)
	for _, vd := range m.videoQueue {
		statusIcon := "…"
		if vd.Done {
			statusIcon = "☑"
		} else if vd.Percent > 0 {
			statusIcon = "▮"
		} else if !vd.TitleFetched {
			statusIcon = "◉"
		}

		bar := progress.New(progress.WithScaledGradient("#F9BE5E", "#d98057"))
		bar.Width = leftWidth - 6

		queueContent += fmt.Sprintf("[%s] %s\n", statusIcon, vd.Name)
		if vd.Percent > 0 || vd.Done {
			queueContent += bar.ViewAs(vd.Percent) + "\n"
		}
	}

	// Completed history
	if len(m.history) == 0 {
		queueContent += lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			Render("\nNO ARCHIVED CASES")
	} else {
		queueContent += "\n"
		for i, info := range m.history {
			prefix := "  "
			if i == m.selectedIndex {
				prefix = "➤ "
			}
			queueContent += fmt.Sprintf("%s%s\n", prefix, truncateString(strings.ToUpper(info.Title), leftWidth-6))
		}
	}

	// Preview box
	previewTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F9BE5E")).
		Render("ARCHIVE PREVIEW")

	previewContent := previewTitle + "\n\n"
	if len(m.history) > 0 {
		info := m.history[m.selectedIndex]
		previewContent += fmt.Sprintf(
			"TITLE: %s\nURL: %s\nDURATION: %.0fs\nRESOLUTION: %s (%dx%d)\nFPS: %d\nVIDEO BITRATE: %.1f kbps\nAUDIO BITRATE: %.1f kbps\nSIZE: %d MB\nDOWNLOADED: %s",
			info.Title,
			info.URL,
			info.Duration,
			info.Resolution, info.Width, info.Height,
			info.FPS,
			info.VBR, info.ABR,
			info.Filesize/1024/1024,
			info.DownloadedAt.Format("2006-01-02 15:04:05"),
		)
	} else {
		previewContent += lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			Render("NO ARCHIVES YET")
	}

	// Input box
	inputTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F9BE5E")).
		Render("NEW CASE ENTRY")

	inputContent := inputTitle + "\n\n" + m.textInput.View()
	inputContent += "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Render("PRESS ENTER TO CONFIRM • ESC TO EXIT • M TO TOGGLE FORMAT: "+strings.ToUpper(m.downloadFormat))

	// Hex vanity box
	hexBoxContent := hexBoxStyle.Render(m.hexStream)

	// top right box
	topRightContent := lipgloss.JoinVertical(
		lipgloss.Right,
		previewBoxStyle.Render(previewContent),
		timelineBoxStyle.Render("timeline"),
	)

	// Status
	statusContent := "\n" + statusStyle.Render("STATUS: "+m.status)

	// Layout
	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		queueBoxStyle.Render(queueContent),
		topRightContent,
	)

	bottomRow := lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		inputBoxStyle.Render(inputContent),
		hexBoxContent,
	)

	return header + "\n\n" + topRow + "\n" + bottomRow + statusContent
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func randomHexString(length int) string {
	const hexChars = "0123456789ABCDEF"
	b := make([]byte, length)
	for i := range b {
		b[i] = hexChars[rand.Intn(len(hexChars))]
	}
	return string(b)
}

func loadHistory(path string) []downloader.VideoInfo {
	data, err := os.ReadFile(path)
	if err != nil {
		return []downloader.VideoInfo{}
	}
	var infos []downloader.VideoInfo
	json.Unmarshal(data, &infos)
	return infos
}

// fetchTitleCmd starts async title fetching
func fetchTitleCmd(url string) tea.Cmd {
	return func() tea.Msg {
		resultCh := make(chan downloader.TitleFetchedMsg, 1)

		downloader.FetchTitleAsync(url, func(msg downloader.TitleFetchedMsg) {
			resultCh <- msg
		})

		select {
		case result := <-resultCh:
			return titleFetchedMsg{
				url:   result.URL,
				title: result.Title,
				err:   result.Error,
			}
		case <-time.After(10 * time.Second):
			return titleFetchedMsg{
				url:   url,
				title: "",
				err:   fmt.Errorf("timeout fetching title"),
			}
		}
	}
}

// startDownloadCmd launches the downloader in a goroutine
func startDownloadCmd(vd *VideoDownload, format string) tea.Cmd {
	return func() tea.Msg {
		downloader.DownloadStreamWithProgress(vd.URL, format, func(f float64, line string) {
			select {
			case vd.ProgressCh <- downloader.ProgressFractionMsg{
				Fraction: f,
				Line:     line,
			}:
			default:
			}
		})
		return nil
	}
}

// tickCmd schedules the next tick
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg { // 10fps
		return tickMsg{}
	})
}

// Helper: generates multi-line formatted hex stream
func formattedHexStream(lines, pairsPerLine int) string {
	const hexChars = "0123456789ABCDEF"
	var result []string

	for i := 0; i < lines; i++ {
		var linePairs []string
		for j := 0; j < pairsPerLine; j++ {
			a := hexChars[rand.Intn(len(hexChars))]
			b := hexChars[rand.Intn(len(hexChars))]
			linePairs = append(linePairs, string(a)+string(b))
		}
		result = append(result, strings.Join(linePairs, " "))
	}

	return strings.Join(result, "\n")
}
