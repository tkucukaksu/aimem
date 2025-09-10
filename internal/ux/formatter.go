package ux

import (
	"fmt"
	"strings"
	"time"

	"github.com/tarkank/aimem/internal/types"
)

// MessageType represents different types of user messages
type MessageType string

const (
	Success  MessageType = "success"
	Error    MessageType = "error"
	Warning  MessageType = "warning"
	Info     MessageType = "info"
	Progress MessageType = "progress"
	Tip      MessageType = "tip"
)

// Formatter provides user-friendly message formatting
type Formatter struct {
	UseEmojis bool
	UseColors bool
}

// NewFormatter creates a new UX formatter
func NewFormatter() *Formatter {
	return &Formatter{
		UseEmojis: true,
		UseColors: false, // Keep simple for MCP protocol
	}
}

// FormatMessage formats messages with appropriate visual cues
func (f *Formatter) FormatMessage(msgType MessageType, message string) string {
	emoji := f.getEmoji(msgType)
	if emoji != "" && f.UseEmojis {
		return fmt.Sprintf("%s %s", emoji, message)
	}
	return message
}

// FormatSessionInfo formats session information in a user-friendly way
func (f *Formatter) FormatSessionInfo(session *types.SessionInfo) string {
	var builder strings.Builder

	builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Session: %s", session.ID)))
	builder.WriteString("\n")

	if projectName, exists := session.Metadata["project_name"].(string); exists {
		builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Project: %s", projectName)))
		builder.WriteString("\n")
	}

	if projectType, exists := session.Metadata["project_type"].(string); exists {
		builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Type: %s", projectType)))
		builder.WriteString("\n")
	}

	if language, exists := session.Metadata["language"].(string); exists {
		if framework, exists := session.Metadata["framework"].(string); exists {
			builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Stack: %s + %s", language, framework)))
		} else {
			builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Language: %s", language)))
		}
		builder.WriteString("\n")
	}

	builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Location: %s", session.WorkingDir)))
	builder.WriteString("\n")
	builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Last Active: %s", f.FormatRelativeTime(session.LastActive))))

	return builder.String()
}

// FormatProjectInfo formats project information
func (f *Formatter) FormatProjectInfo(project *types.ProjectInfo) string {
	var builder strings.Builder

	builder.WriteString(f.FormatMessage(Success, fmt.Sprintf("Detected Project: %s", project.Name)))
	builder.WriteString("\n")
	builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Type: %s", project.Type)))
	builder.WriteString("\n")

	if project.Language != "" {
		if project.Framework != "" {
			builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Stack: %s + %s", project.Language, project.Framework)))
		} else {
			builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Language: %s", project.Language)))
		}
		builder.WriteString("\n")
	}

	builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Location: %s", project.CanonicalPath)))

	if project.GitRemote != nil {
		builder.WriteString("\n")
		builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Repository: %s", *project.GitRemote)))
	}

	if len(project.WorkspaceMarkers) > 0 {
		builder.WriteString("\n")
		builder.WriteString(f.FormatMessage(Info, fmt.Sprintf("Workspace Markers: %s", strings.Join(project.WorkspaceMarkers, ", "))))
	}

	return builder.String()
}

// FormatPerformanceMetrics formats performance metrics in a readable way
func (f *Formatter) FormatPerformanceMetrics(metrics map[string]interface{}) string {
	var builder strings.Builder

	builder.WriteString(f.FormatMessage(Info, "Performance Snapshot:"))
	builder.WriteString("\n")

	if enabled, exists := metrics["enabled"].(bool); exists && !enabled {
		builder.WriteString(f.FormatMessage(Warning, "Performance monitoring is disabled"))
		return builder.String()
	}

	if uptime, exists := metrics["uptime_seconds"].(float64); exists {
		builder.WriteString(fmt.Sprintf("• Uptime: %s", f.FormatDuration(time.Duration(uptime)*time.Second)))
		builder.WriteString("\n")
	}

	if reqCount, exists := metrics["total_requests"].(int64); exists {
		builder.WriteString(fmt.Sprintf("• Total Requests: %s", f.FormatNumber(reqCount)))
		builder.WriteString("\n")
	}

	if avgLatency, exists := metrics["average_latency_ms"].(int64); exists {
		status := f.getLatencyStatus(avgLatency)
		builder.WriteString(fmt.Sprintf("• Response Time: %s (%dms)", status, avgLatency))
		builder.WriteString("\n")
	}

	if rps, exists := metrics["requests_per_second"].(float64); exists {
		builder.WriteString(fmt.Sprintf("• Throughput: %.1f req/sec", rps))
		builder.WriteString("\n")
	}

	if errorRate, exists := metrics["error_rate_percent"].(float64); exists {
		status := f.getErrorRateStatus(errorRate)
		builder.WriteString(fmt.Sprintf("• Error Rate: %s (%.2f%%)", status, errorRate))
		builder.WriteString("\n")
	}

	if sessions, exists := metrics["active_sessions"].(int); exists {
		builder.WriteString(fmt.Sprintf("• Active Sessions: %d", sessions))
	}

	return builder.String()
}

// FormatMemoryUsage formats memory usage information
func (f *Formatter) FormatMemoryUsage(used int64, limit int64, chunkCount int64) string {
	var builder strings.Builder

	usedMB := float64(used) / 1024 / 1024
	limitMB := float64(limit) / 1024 / 1024
	percentage := float64(used) / float64(limit) * 100

	status := f.getMemoryStatus(percentage)

	builder.WriteString(f.FormatMessage(Info, "Memory Status:"))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("• Usage: %s (%.1fMB / %.1fMB, %.1f%%)", status, usedMB, limitMB, percentage))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("• Chunks: %d", chunkCount))

	if percentage > 80 {
		builder.WriteString("\n")
		builder.WriteString(f.FormatMessage(Tip, "Consider running session cleanup to free memory"))
	}

	return builder.String()
}

// FormatError formats error messages with suggestions
func (f *Formatter) FormatError(err error, suggestions ...string) string {
	var builder strings.Builder

	builder.WriteString(f.FormatMessage(Error, err.Error()))

	for _, suggestion := range suggestions {
		builder.WriteString("\n")
		builder.WriteString(f.FormatMessage(Tip, suggestion))
	}

	return builder.String()
}

// FormatProgressUpdate formats progress updates
func (f *Formatter) FormatProgressUpdate(operation string, current, total int) string {
	if total > 0 {
		percentage := float64(current) / float64(total) * 100
		return f.FormatMessage(Progress, fmt.Sprintf("%s (%d/%d, %.1f%%)", operation, current, total, percentage))
	}
	return f.FormatMessage(Progress, fmt.Sprintf("%s (%d completed)", operation, current))
}

// FormatList formats a list of items with bullets
func (f *Formatter) FormatList(title string, items []string, numbered bool) string {
	var builder strings.Builder

	if title != "" {
		builder.WriteString(f.FormatMessage(Info, title))
		builder.WriteString("\n")
	}

	for i, item := range items {
		if numbered {
			builder.WriteString(fmt.Sprintf("  %d. %s", i+1, item))
		} else {
			builder.WriteString(fmt.Sprintf("  • %s", item))
		}
		if i < len(items)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// Helper methods

func (f *Formatter) getEmoji(msgType MessageType) string {
	if !f.UseEmojis {
		return ""
	}

	switch msgType {
	case Success:
		return "✅"
	case Error:
		return "❌"
	case Warning:
		return "⚠️"
	case Info:
		return "ℹ️"
	case Progress:
		return "🔄"
	case Tip:
		return "💡"
	default:
		return ""
	}
}

func (f *Formatter) getLatencyStatus(latencyMs int64) string {
	switch {
	case latencyMs < 20:
		return "Excellent"
	case latencyMs < 50:
		return "Good"
	case latencyMs < 100:
		return "Fair"
	default:
		return "Needs Improvement"
	}
}

func (f *Formatter) getErrorRateStatus(errorRate float64) string {
	switch {
	case errorRate < 0.1:
		return "Excellent"
	case errorRate < 1.0:
		return "Good"
	case errorRate < 5.0:
		return "Fair"
	default:
		return "Needs Attention"
	}
}

func (f *Formatter) getMemoryStatus(percentage float64) string {
	switch {
	case percentage < 50:
		return "Healthy"
	case percentage < 70:
		return "Moderate"
	case percentage < 85:
		return "High"
	default:
		return "Critical"
	}
}

func (f *Formatter) FormatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2, 2006")
	}
}

func (f *Formatter) FormatDuration(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	}
	if duration < time.Hour {
		return fmt.Sprintf("%.1fm", duration.Minutes())
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
	days := duration.Hours() / 24
	return fmt.Sprintf("%.1fd", days)
}

func (f *Formatter) FormatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

// GetWelcomeMessage returns a welcome message for new users
func (f *Formatter) GetWelcomeMessage() string {
	var builder strings.Builder

	builder.WriteString(f.FormatMessage(Success, "Welcome to AIMem!"))
	builder.WriteString("\n\n")

	tips := []string{
		"AIMem automatically detects your project type and structure",
		"Context is stored automatically during conversations",
		"Use natural language to search through your context",
		"Sessions are organized by project for better memory management",
	}

	builder.WriteString(f.FormatList("💡 Quick Start Tips:", tips, true))

	return builder.String()
}

// GetOptimizationTips returns performance optimization suggestions
func (f *Formatter) GetOptimizationTips(sessionStats *types.SessionSummary) []string {
	var tips []string

	if sessionStats.ChunkCount > 100 {
		tips = append(tips, "Consider running cleanup to remove old chunks")
	}

	if sessionStats.MemoryUsage > 50*1024*1024 { // 50MB
		tips = append(tips, "Session memory usage is high - cleanup recommended")
	}

	if time.Since(sessionStats.LastActivity) > 24*time.Hour {
		tips = append(tips, "Session hasn't been active recently - archive if no longer needed")
	}

	if sessionStats.AverageRelevance < 0.6 {
		tips = append(tips, "Low average relevance - consider storing more focused context")
	}

	return tips
}
