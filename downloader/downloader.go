package downloader

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Message sent from downloader to TUI
type ProgressFractionMsg struct {
	Fraction float64
	Line     string
}

// TitleFetchedMsg is sent when title is fetched
type TitleFetchedMsg struct {
	URL   string
	Title string
	Error error
}

// ProgressCallback is called with (fraction, logLine)
type ProgressCallback func(fraction float64, logLine string)

// TitleCallback is called when title is fetched
type TitleCallback func(TitleFetchedMsg)

// VideoInfo represents saved metadata
type VideoInfo struct {
	URL          string    `json:"url"`
	Title        string    `json:"title"`
	Duration     float64   `json:"duration"`
	Resolution   string    `json:"resolution"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	FPS          int       `json:"fps"`
	VBR          float64   `json:"video_bitrate_kbps"`
	ABR          float64   `json:"audio_bitrate_kbps"`
	TBR          float64   `json:"total_bitrate_kbps"`
	Filesize     int64     `json:"filesize"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// DownloadStreamWithProgress streams video download progress via callback
func DownloadStreamWithProgress(url string, callback ProgressCallback) {
	go func() {
		cmd := exec.Command(
			"yt-dlp",
			"-f", "bestvideo[height<=2160]+bestaudio/best",
			"--merge-output-format", "mp4",
			"--newline",
			url,
		)

		stderr, err := cmd.StderrPipe()
		if err != nil {
			callback(0, "❌ Error creating stderr pipe: "+err.Error())
			return
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			callback(0, "❌ Error creating stdout pipe: "+err.Error())
			return
		}

		if err := cmd.Start(); err != nil {
			callback(0, "❌ Error starting download: "+err.Error())
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			readOutput(stderr, callback, "stderr")
		}()

		go func() {
			defer wg.Done()
			readOutput(stdout, callback, "stdout")
		}()

		wg.Wait()

		if err := cmd.Wait(); err != nil {
			callback(1.0, "❌ Download failed: "+err.Error())
		} else {
			callback(1.0, "✅ Variant pruned - Timeline restored!")

			// ✅ Save metadata after successful download
			saveVideoInfo(url, "downloads.json")
		}

		// ✅ Final step: tell caller to close channel
		callback(1.0, "")
	}()
}

// readOutput reads from a pipe and processes the output
func readOutput(r io.Reader, callback ProgressCallback, source string) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse download progress
		fraction := parseProgress(line)

		// Send the progress update
		callback(fraction, line)
	}

	if err := scanner.Err(); err != nil {
		callback(-1, "❌ Error reading "+source+": "+err.Error())
	}
}

// parseProgress extracts progress percentage from yt-dlp output
func parseProgress(line string) float64 {
	// Look for download percentage
	downloadRegex := regexp.MustCompile(`\[download\]\s+(\d+(?:\.\d+)?)%`)
	if matches := downloadRegex.FindStringSubmatch(line); len(matches) > 1 {
		if percent, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return percent / 100.0
		}
	}

	// Fallback to stage-based progress
	line = strings.ToLower(line)

	if strings.Contains(line, "extracting") || strings.Contains(line, "downloading webpage") {
		return 0.05
	}
	if strings.Contains(line, "[download]") && strings.Contains(line, "%") {
		return 0.3 // Default for active download
	}
	if strings.Contains(line, "downloading video") {
		return 0.4
	}
	if strings.Contains(line, "downloading audio") {
		return 0.7
	}
	if strings.Contains(line, "merging") || strings.Contains(line, "post-processing") {
		return 0.9
	}
	if strings.Contains(line, "finished") || strings.Contains(line, "completed") {
		return 1.0
	}

	return -1 // No progress detected, keep current progress
}

// FetchTitleAsync fetches video title asynchronously
func FetchTitleAsync(url string, callback TitleCallback) {
	go func() {
		// Add timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "yt-dlp", "--get-title", url)
		var out bytes.Buffer
		var errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut

		err := cmd.Run()

		title := ""
		if err == nil {
			title = strings.TrimSpace(out.String())
		}

		// If title fetch failed, use URL as fallback
		if title == "" {
			if err != nil {
				title = extractURLName(url)
				callback(TitleFetchedMsg{
					URL:   url,
					Title: title,
					Error: err,
				})
			} else {
				title = extractURLName(url)
				callback(TitleFetchedMsg{
					URL:   url,
					Title: title,
					Error: nil,
				})
			}
		} else {
			callback(TitleFetchedMsg{
				URL:   url,
				Title: title,
				Error: nil,
			})
		}
	}()
}

// FetchTitle synchronous version (kept for backward compatibility, but not recommended)
func FetchTitle(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp", "--get-title", url)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return extractURLName(url), err
	}

	title := strings.TrimSpace(out.String())
	if title == "" {
		return extractURLName(url), nil
	}

	return title, nil
}

// extractURLName extracts a reasonable name from URL as fallback
func extractURLName(url string) string {
	// Remove protocol
	if idx := strings.Index(url, "://"); idx != -1 {
		url = url[idx+3:]
	}

	// Extract video ID from common patterns
	if strings.Contains(url, "youtube.com/watch?v=") {
		if idx := strings.Index(url, "v="); idx != -1 {
			videoID := url[idx+2:]
			if idx := strings.Index(videoID, "&"); idx != -1 {
				videoID = videoID[:idx]
			}
			return "YouTube Video: " + videoID
		}
	} else if strings.Contains(url, "youtu.be/") {
		if idx := strings.Index(url, "youtu.be/"); idx != -1 {
			videoID := url[idx+9:]
			if idx := strings.Index(videoID, "?"); idx != -1 {
				videoID = videoID[:idx]
			}
			return "YouTube Video: " + videoID
		}
	}

	// Generic fallback
	if len(url) > 50 {
		return url[:47] + "..."
	}
	return url
}

// saveVideoInfo appends metadata to downloads.json
func saveVideoInfo(url string, path string) {
	cmd := exec.Command("yt-dlp", "--dump-json", "-f", "bestvideo+bestaudio/best", url)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return // skip if metadata fetch fails
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		return
	}

	info := VideoInfo{
		URL:          url,
		Title:        raw["title"].(string),
		DownloadedAt: time.Now(),
	}

	if d, ok := raw["duration"].(float64); ok {
		info.Duration = d
	}
	if r, ok := raw["resolution"].(string); ok {
		info.Resolution = r
	}
	if s, ok := raw["filesize"].(float64); ok {
		info.Filesize = int64(s)
	}

	if w, ok := raw["width"].(float64); ok {
		info.Width = int(w)
	}
	if h, ok := raw["height"].(float64); ok {
		info.Height = int(h)
	}
	if f, ok := raw["fps"].(float64); ok {
		info.FPS = int(f)
	}
	if v, ok := raw["vbr"].(float64); ok {
		info.VBR = v
	}
	if a, ok := raw["abr"].(float64); ok {
		info.ABR = a
	}
	if t, ok := raw["tbr"].(float64); ok {
		info.TBR = t
	}

	var infos []VideoInfo
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &infos)
	}

	infos = append(infos, info)

	if data, err := json.MarshalIndent(infos, "", "  "); err == nil {
		os.WriteFile(path, data, 0644)
	}
}

// Helper functions (kept for compatibility)
func lineContains(line, substr string) bool {
	return strings.Contains(strings.ToLower(line), strings.ToLower(substr))
}

func contains(s, substr string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(substr))
}
