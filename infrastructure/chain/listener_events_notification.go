package chain

import "fmt"

// MiniAppNotificationEvent represents a notification from a MiniApp contract.
// Event: Notification(appId, title, content, notificationType, priority)
type MiniAppNotificationEvent struct {
	AppID            string
	Title            string
	Content          string
	NotificationType string
	Priority         int
}

func ParseMiniAppNotificationEvent(event *ContractEvent) (*MiniAppNotificationEvent, error) {
	if event.EventName != "Notification" {
		return nil, fmt.Errorf("not a Notification event")
	}
	if len(event.State) < 4 {
		return nil, fmt.Errorf("invalid event state: expected at least 4 items, got %d", len(event.State))
	}

	appID, err := ParseStringFromItem(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse appId: %w", err)
	}

	title, err := ParseStringFromItem(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse title: %w", err)
	}

	content, err := ParseStringFromItem(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse content: %w", err)
	}

	notifType, err := ParseStringFromItem(event.State[3])
	if err != nil {
		notifType = "news"
	}

	priority := 0
	if len(event.State) >= 5 {
		if p, err := ParseInteger(event.State[4]); err == nil {
			priority = int(p.Int64())
		}
	}

	return &MiniAppNotificationEvent{
		AppID:            appID,
		Title:            title,
		Content:          content,
		NotificationType: notifType,
		Priority:         priority,
	}, nil
}
