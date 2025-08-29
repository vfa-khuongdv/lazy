package notification

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vfa-khuongdv/go-backup-drive/internal/database"
)

// Manager handles multiple notification channels and message formatting
type Manager struct {
	dbService *database.Service
	notifiers map[string]Notifier
	mutex     sync.RWMutex
}

// NewManager creates a new notification manager
func NewManager(dbService *database.Service) *Manager {
	return &Manager{
		dbService: dbService,
		notifiers: make(map[string]Notifier),
	}
}

// AddNotifier adds a notifier instance for a specific configuration
func (m *Manager) AddNotifier(configName string, notifier Notifier) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.notifiers[configName] = notifier
	log.Printf("Added %s notifier for config '%s'", notifier.GetChannelType(), configName)
}

// RemoveNotifier removes a notifier for a specific configuration
func (m *Manager) RemoveNotifier(configName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.notifiers, configName)
	log.Printf("Removed notifier for config '%s'", configName)
}

// SendNotification sends a notification to all configured channels
func (m *Manager) SendNotification(message *Message) []NotificationResult {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var results []NotificationResult
	var wg sync.WaitGroup
	resultChan := make(chan NotificationResult, len(m.notifiers))

	// Send notifications concurrently
	for configName, notifier := range m.notifiers {
		wg.Add(1)
		go func(name string, n Notifier) {
			defer wg.Done()

			result := NotificationResult{
				Channel: n.GetChannelType(),
				SentAt:  time.Now(),
			}

			err := n.Send(message)
			if err != nil {
				result.Success = false
				result.Error = err.Error()
				log.Printf("Failed to send notification via %s (config: %s): %v", n.GetChannelType(), name, err)
			} else {
				result.Success = true
				log.Printf("Successfully sent notification via %s (config: %s)", n.GetChannelType(), name)
			}

			resultChan <- result
		}(configName, notifier)
	}

	// Wait for all notifications to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// SendBackupSuccessNotification sends a backup success notification
func (m *Manager) SendBackupSuccessNotification(data *BackupNotificationData) []NotificationResult {
	// Get all enabled notification configs
	configs, err := m.getEnabledNotificationConfigs()
	if err != nil {
		log.Printf("Failed to get notification configs: %v", err)
		return nil
	}

	var allResults []NotificationResult

	for _, config := range configs {
		if !config.NotifyOnSuccess {
			continue
		}

		// Create channel-specific message
		var message *Message
		switch config.Channel {
		case "discord":
			message = CreateDiscordBackupSuccessMessage(data)
		case "slack":
			message = CreateSlackBackupSuccessMessage(data)
		case "chatwork":
			message = CreateBackupSuccessMessage(data)
		default:
			message = CreateBackupSuccessMessage(data)
		}

		// Create notifier if not exists
		notifier, err := m.createNotifierFromConfig(&config)
		if err != nil {
			log.Printf("Failed to create notifier for config '%s': %v", config.Name, err)
			continue
		}

		// Send notification
		result := NotificationResult{
			Channel: notifier.GetChannelType(),
			SentAt:  time.Now(),
		}

		err = notifier.Send(message)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			log.Printf("Failed to send success notification via %s (config: %s): %v", config.Channel, config.Name, err)
		} else {
			result.Success = true
			log.Printf("Successfully sent success notification via %s (config: %s)", config.Channel, config.Name)
		}

		allResults = append(allResults, result)
	}

	return allResults
}

// SendBackupErrorNotification sends a backup error notification
func (m *Manager) SendBackupErrorNotification(data *BackupNotificationData) []NotificationResult {
	// Get all enabled notification configs
	configs, err := m.getEnabledNotificationConfigs()
	if err != nil {
		log.Printf("Failed to get notification configs: %v", err)
		return nil
	}

	var allResults []NotificationResult

	for _, config := range configs {
		if !config.NotifyOnError {
			continue
		}

		// Create channel-specific message
		var message *Message
		switch config.Channel {
		case "discord":
			message = CreateDiscordBackupErrorMessage(data)
		case "slack":
			message = CreateSlackBackupErrorMessage(data)
		case "chatwork":
			message = CreateBackupErrorMessage(data)
		default:
			message = CreateBackupErrorMessage(data)
		}

		// Create notifier if not exists
		notifier, err := m.createNotifierFromConfig(&config)
		if err != nil {
			log.Printf("Failed to create notifier for config '%s': %v", config.Name, err)
			continue
		}

		// Send notification
		result := NotificationResult{
			Channel: notifier.GetChannelType(),
			SentAt:  time.Now(),
		}

		err = notifier.Send(message)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			log.Printf("Failed to send error notification via %s (config: %s): %v", config.Channel, config.Name, err)
		} else {
			result.Success = true
			log.Printf("Successfully sent error notification via %s (config: %s)", config.Channel, config.Name)
		}

		allResults = append(allResults, result)
	}

	return allResults
}

// TestNotification sends a test notification to a specific channel
func (m *Manager) TestNotification(configName string) error {
	config, err := m.dbService.GetNotificationConfigByName(configName)
	if err != nil {
		return fmt.Errorf("failed to get notification config: %w", err)
	}

	// Create notifier
	notifier, err := m.createNotifierFromConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create notifier: %w", err)
	}

	// Create test message
	message := &Message{
		Type:      MessageTypeInfo,
		Title:     "Test Notification",
		Text:      fmt.Sprintf("This is a test notification from the Database Backup Service via %s", config.Channel),
		Timestamp: time.Now(),
		Fields: map[string]interface{}{
			"Channel":       string(config.Channel),
			"Configuration": configName,
			"Test Status":   "Success",
		},
	}

	// Send notification
	return notifier.Send(message)
}

// getEnabledNotificationConfigs returns all enabled notification configurations
func (m *Manager) getEnabledNotificationConfigs() ([]database.NotificationConfig, error) {
	return m.dbService.GetEnabledNotificationConfigs()
}

// createNotifierFromConfig creates a notifier instance from configuration
func (m *Manager) createNotifierFromConfig(config *database.NotificationConfig) (Notifier, error) {
	// Convert string channel to NotificationChannel type
	var channel NotificationChannel
	switch config.Channel {
	case "chatwork":
		channel = ChannelChatwork
	case "discord":
		channel = ChannelDiscord
	case "slack":
		channel = ChannelSlack
	default:
		return nil, fmt.Errorf("unsupported notification channel: %s", config.Channel)
	}

	switch channel {
	case ChannelChatwork:
		chatworkConfig, err := m.parseChatworkConfig(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Chatwork config: %w", err)
		}
		return NewChatworkNotifier(*chatworkConfig), nil

	case ChannelDiscord:
		discordConfig, err := m.parseDiscordConfig(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Discord config: %w", err)
		}
		return NewDiscordNotifier(*discordConfig), nil

	case ChannelSlack:
		slackConfig, err := m.parseSlackConfig(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Slack config: %w", err)
		}
		return NewSlackNotifier(*slackConfig), nil

	default:
		return nil, fmt.Errorf("unsupported notification channel: %s", config.Channel)
	}
}

// parseChatworkConfig parses configuration map to ChatworkConfig
func (m *Manager) parseChatworkConfig(config map[string]interface{}) (*ChatworkConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var chatworkConfig ChatworkConfig
	err = json.Unmarshal(jsonData, &chatworkConfig)
	if err != nil {
		return nil, err
	}

	return &chatworkConfig, nil
}

// parseDiscordConfig parses configuration map to DiscordConfig
func (m *Manager) parseDiscordConfig(config map[string]interface{}) (*DiscordConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var discordConfig DiscordConfig
	err = json.Unmarshal(jsonData, &discordConfig)
	if err != nil {
		return nil, err
	}

	return &discordConfig, nil
}

// parseSlackConfig parses configuration map to SlackConfig
func (m *Manager) parseSlackConfig(config map[string]interface{}) (*SlackConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var slackConfig SlackConfig
	err = json.Unmarshal(jsonData, &slackConfig)
	if err != nil {
		return nil, err
	}

	return &slackConfig, nil
}

// LoadNotifiers loads all notification configurations and creates notifiers
func (m *Manager) LoadNotifiers() error {
	configs, err := m.getEnabledNotificationConfigs()
	if err != nil {
		return fmt.Errorf("failed to get notification configs: %w", err)
	}

	for _, config := range configs {
		notifier, err := m.createNotifierFromConfig(&config)
		if err != nil {
			log.Printf("Failed to create notifier for config '%s': %v", config.Name, err)
			continue
		}

		m.AddNotifier(config.Name, notifier)
	}

	log.Printf("Loaded %d notification channels", len(configs))
	return nil
}

// GetNotifierCount returns the number of active notifiers
func (m *Manager) GetNotifierCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.notifiers)
}
