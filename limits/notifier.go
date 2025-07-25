package limits

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/penwyp/ClawCat/config"
)

// Notifier 通知器
type Notifier struct {
	config       *config.Config
	enabledTypes []config.NotificationType
}

// NewNotifier 创建通知器
func NewNotifier(cfg *config.Config) *Notifier {
	enabledTypes := []config.NotificationType{config.NotifyDesktop} // 默认启用桌面通知
	
	if cfg != nil && cfg.Limits.Notifications != nil {
		enabledTypes = cfg.Limits.Notifications
	}

	return &Notifier{
		config:       cfg,
		enabledTypes: enabledTypes,
	}
}

// SendNotification 发送通知
func (n *Notifier) SendNotification(message string, severity Severity) error {
	var errors []error

	for _, notifType := range n.enabledTypes {
		var err error

		switch notifType {
		case config.NotifyDesktop:
			err = n.sendDesktopNotification(message, severity)
		case config.NotifySound:
			err = n.playSound(severity)
		case config.NotifyWebhook:
			err = n.sendWebhookNotification(message, severity)
		case config.NotifyEmail:
			err = n.sendEmailNotification(message, severity)
		}

		if err != nil {
			errors = append(errors, fmt.Errorf("%s notification failed: %w", notifType, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("notification errors: %v", errors)
	}

	return nil
}

// sendDesktopNotification 发送桌面通知
func (n *Notifier) sendDesktopNotification(message string, severity Severity) error {
	title := fmt.Sprintf("ClawCat - %s", n.capitalizeFirst(string(severity)))

	switch runtime.GOOS {
	case "darwin":
		// macOS
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
		return exec.Command("osascript", "-e", script).Run()

	case "linux":
		// Linux (需要 notify-send)
		icon := n.getIconForSeverity(severity)
		return exec.Command("notify-send", "-i", icon, title, message).Run()

	case "windows":
		// Windows (使用 PowerShell)
		ps := fmt.Sprintf(`
			Add-Type -AssemblyName System.Windows.Forms
			$notification = New-Object System.Windows.Forms.NotifyIcon
			$notification.Icon = [System.Drawing.SystemIcons]::Information
			$notification.BalloonTipTitle = "%s"
			$notification.BalloonTipText = "%s"
			$notification.Visible = $true
			$notification.ShowBalloonTip(10000)
		`, title, message)
		return exec.Command("powershell", "-Command", ps).Run()

	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// playSound 播放提示音
func (n *Notifier) playSound(severity Severity) error {
	soundFile := n.getSoundForSeverity(severity)

	switch runtime.GOOS {
	case "darwin":
		if soundFile == "" {
			// 使用系统默认声音
			return exec.Command("afplay", "/System/Library/Sounds/Glass.aiff").Run()
		}
		return exec.Command("afplay", soundFile).Run()
	case "linux":
		if soundFile == "" {
			// 使用系统提示音
			return exec.Command("paplay", "/usr/share/sounds/alsa/Front_Left.wav").Run()
		}
		return exec.Command("paplay", soundFile).Run()
	case "windows":
		// Windows 系统声音
		ps := `[console]::beep(800,300)`
		return exec.Command("powershell", "-c", ps).Run()
	default:
		return nil
	}
}

// sendWebhookNotification 发送 Webhook 通知
func (n *Notifier) sendWebhookNotification(message string, severity Severity) error {
	webhookURL := ""
	if n.config != nil {
		webhookURL = n.config.Limits.WebhookURL
	}
	
	if webhookURL == "" {
		return nil // 没有配置 webhook，跳过
	}

	payload := map[string]interface{}{
		"message":   message,
		"severity":  severity,
		"timestamp": time.Now().Unix(),
		"source":    "clawcat",
		"type":      "limit_warning",
	}

	// 如果有当前使用信息，也包含在内
	if n.config != nil {
		payload["usage"] = map[string]interface{}{
			"plan": n.config.Subscription.Plan,
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// sendEmailNotification 发送邮件通知
func (n *Notifier) sendEmailNotification(message string, severity Severity) error {
	// TODO: 实现邮件通知
	// 这里可以集成 SMTP 或第三方邮件服务
	fmt.Printf("Email notification: [%s] %s\n", severity, message)
	return nil
}

// getIconForSeverity 根据严重程度获取图标
func (n *Notifier) getIconForSeverity(severity Severity) string {
	switch severity {
	case SeverityInfo:
		return "dialog-information"
	case SeverityWarning:
		return "dialog-warning"
	case SeverityError:
		return "dialog-error"
	case SeverityCritical:
		return "dialog-error"
	default:
		return "dialog-information"
	}
}

// getSoundForSeverity 根据严重程度获取声音文件
func (n *Notifier) getSoundForSeverity(severity Severity) string {
	// 返回空字符串将使用系统默认声音
	// 实际项目中可以配置自定义声音文件路径
	return ""
}

// capitalizeFirst 首字母大写
func (n *Notifier) capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// SetEnabledTypes 设置启用的通知类型
func (n *Notifier) SetEnabledTypes(types []config.NotificationType) {
	n.enabledTypes = types
}

// IsEnabled 检查特定通知类型是否启用
func (n *Notifier) IsEnabled(notifType config.NotificationType) bool {
	for _, t := range n.enabledTypes {
		if t == notifType {
			return true
		}
	}
	return false
}

// TestNotification 测试通知功能
func (n *Notifier) TestNotification() error {
	testMessage := "ClawCat notification test - all systems working!"
	return n.SendNotification(testMessage, SeverityInfo)
}

// NotificationHistory 通知历史记录
type NotificationHistory struct {
	Message   string    `json:"message"`
	Severity  Severity  `json:"severity"`
	Type      []config.NotificationType `json:"types"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// Enhanced Notifier with history tracking
type EnhancedNotifier struct {
	*Notifier
	history []NotificationHistory
	maxHistory int
}

// NewEnhancedNotifier 创建增强通知器
func NewEnhancedNotifier(cfg *config.Config) *EnhancedNotifier {
	return &EnhancedNotifier{
		Notifier:   NewNotifier(cfg),
		history:    make([]NotificationHistory, 0),
		maxHistory: 100, // 保留最近100条通知记录
	}
}

// SendNotificationWithHistory 发送通知并记录历史
func (en *EnhancedNotifier) SendNotificationWithHistory(message string, severity Severity) error {
	startTime := time.Now()
	err := en.Notifier.SendNotification(message, severity)
	
	// 记录到历史
	record := NotificationHistory{
		Message:   message,
		Severity:  severity,
		Type:      en.enabledTypes,
		Timestamp: startTime,
		Success:   err == nil,
	}
	
	if err != nil {
		record.Error = err.Error()
	}
	
	en.addToHistory(record)
	
	return err
}

// addToHistory 添加到历史记录
func (en *EnhancedNotifier) addToHistory(record NotificationHistory) {
	en.history = append(en.history, record)
	
	// 限制历史记录数量
	if len(en.history) > en.maxHistory {
		en.history = en.history[len(en.history)-en.maxHistory:]
	}
}

// GetHistory 获取通知历史
func (en *EnhancedNotifier) GetHistory() []NotificationHistory {
	return en.history
}

// GetRecentFailures 获取最近的失败通知
func (en *EnhancedNotifier) GetRecentFailures(since time.Time) []NotificationHistory {
	failures := []NotificationHistory{}
	
	for _, record := range en.history {
		if !record.Success && record.Timestamp.After(since) {
			failures = append(failures, record)
		}
	}
	
	return failures
}

// GetNotificationStats 获取通知统计
func (en *EnhancedNotifier) GetNotificationStats() map[string]interface{} {
	if len(en.history) == 0 {
		return map[string]interface{}{
			"total": 0,
			"success_rate": 0.0,
		}
	}
	
	total := len(en.history)
	successful := 0
	severityCount := make(map[Severity]int)
	
	for _, record := range en.history {
		if record.Success {
			successful++
		}
		severityCount[record.Severity]++
	}
	
	successRate := float64(successful) / float64(total) * 100
	
	return map[string]interface{}{
		"total":           total,
		"successful":      successful,
		"failed":          total - successful,
		"success_rate":    successRate,
		"by_severity":     severityCount,
		"last_notification": en.history[len(en.history)-1].Timestamp,
	}
}