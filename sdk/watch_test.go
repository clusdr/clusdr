package clusdr

import "testing"

func TestNormalizeWatchTopics(t *testing.T) {
	got, err := normalizeWatchTopics([]string{" deployment ", "custom.deployment", "alerts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "deployment" || got[1] != "alerts" {
		t.Fatalf("%q", got)
	}
	if _, err := normalizeWatchTopics([]string{"bad topic"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyWatchOptions(t *testing.T) {
	o, err := applyWatchOptions([]WatchOption{WithTopics("ping"), WithEventTypes("watch.sync", "watch.sync")})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.topics) != 1 || o.topics[0] != "ping" {
		t.Fatalf("topics %q", o.topics)
	}
	if len(o.eventTypes) != 1 || o.eventTypes[0] != "watch.sync" {
		t.Fatalf("types %q", o.eventTypes)
	}
}
