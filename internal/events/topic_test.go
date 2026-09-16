package events_test

import (
	"strings"
	"testing"

	"github.com/clusdr/clusdr/internal/events"
)

func TestTopicFromType(t *testing.T) {
	topic, ok := events.TopicFromType("custom.deployment")
	if !ok || topic != "deployment" {
		t.Errorf("got %q %v", topic, ok)
	}
	if _, ok := events.TopicFromType("member.join"); ok {
		t.Error("member.join should not parse as a topic")
	}
}

func TestNormalizeTopic(t *testing.T) {
	if got := events.NormalizeTopic("custom.deployment"); got != "deployment" {
		t.Errorf("got %q", got)
	}
	if got := events.NormalizeTopic("deployment"); got != "deployment" {
		t.Errorf("got %q", got)
	}
}

func TestValidTopic(t *testing.T) {
	ok := []string{"deployment", "app.rollout", "worker_1", "a-b"}
	for _, topic := range ok {
		if err := events.ValidTopic(topic); err != nil {
			t.Errorf("%q: %v", topic, err)
		}
	}

	bad := []string{"", "has space", "slash/no", strings.Repeat("x", events.MaxTopicLen+1)}
	for _, topic := range bad {
		if err := events.ValidTopic(topic); err == nil {
			t.Errorf("%q: expected error", topic)
		}
	}
}
