package clusdr

import (
	"fmt"
	"strings"
	"unicode"
)

const maxWatchTopicLen = 128

type watchOptions struct {
	topics     []string
	eventTypes []string
}

// WatchOption configures Watch. Zero options is the full bus (membership
// snapshot, then live events).
type WatchOption func(*watchOptions)

// WithTopics restricts Watch to custom.<topic> for the listed keys
// (with or without a "custom." prefix). Membership snapshot is omitted.
// Custom events are still not replayed on reconnect. Empty/omitted topics
// leave the stream unfiltered. Reconnects reuse the same filter.
func WithTopics(topics ...string) WatchOption {
	return func(o *watchOptions) {
		o.topics = append(o.topics, topics...)
	}
}

// WithEventTypes restricts live events to these full type strings
// (member.join, custom.deployment, …). Protocol events watch.sync and
// watch.gap always pass. Combines with WithTopics (both must match).
func WithEventTypes(types ...string) WatchOption {
	return func(o *watchOptions) {
		o.eventTypes = append(o.eventTypes, types...)
	}
}

func applyWatchOptions(opts []WatchOption) (watchOptions, error) {
	var o watchOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	topics, err := normalizeWatchTopics(o.topics)
	if err != nil {
		return watchOptions{}, err
	}
	o.topics = topics
	o.eventTypes = normalizeWatchTypes(o.eventTypes)
	return o, nil
}

func normalizeWatchTopics(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		t := strings.TrimPrefix(strings.TrimSpace(raw), "custom.")
		if t == "" {
			continue
		}
		if err := validWatchTopic(t); err != nil {
			return nil, fmt.Errorf("clusdr: watch topic %q: %w", raw, err)
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out, nil
}

func normalizeWatchTypes(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func validWatchTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	if len(topic) > maxWatchTopicLen {
		return fmt.Errorf("topic exceeds %d characters", maxWatchTopicLen)
	}
	for _, r := range topic {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' && r != '-' {
			return fmt.Errorf("topic contains invalid character %q", r)
		}
	}
	return nil
}
