package events

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	// MaxTopicLen is the maximum length of a custom event topic.
	MaxTopicLen = 128
	// MaxPayloadBytes is the maximum opaque payload size (64 KiB).
	// Custom events are coordination signals, not a message bus.
	MaxPayloadBytes = 64 << 10
)

// CustomType returns the Watch event type for a user topic: "custom.<topic>".
func CustomType(topic string) string {
	return TypeCustomPrefix + topic
}

// NormalizeTopic strips a leading "custom." prefix so callers can pass either
// "deployment" or "custom.deployment".
func NormalizeTopic(topic string) string {
	return strings.TrimPrefix(topic, TypeCustomPrefix)
}

// TopicFromType extracts the topic from a "custom.<topic>" event type.
func TopicFromType(eventType string) (topic string, ok bool) {
	if !strings.HasPrefix(eventType, TypeCustomPrefix) {
		return "", false
	}
	topic = eventType[len(TypeCustomPrefix):]
	if topic == "" {
		return "", false
	}
	return topic, true
}

// ValidTopic reports whether topic is a legal custom-event routing key.
func ValidTopic(topic string) error {
	topic = NormalizeTopic(topic)
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	if len(topic) > MaxTopicLen {
		return fmt.Errorf("topic exceeds %d characters", MaxTopicLen)
	}
	for _, r := range topic {
		if !isTopicChar(r) {
			return fmt.Errorf("topic contains invalid character %q", r)
		}
	}
	return nil
}

func isTopicChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-'
}
