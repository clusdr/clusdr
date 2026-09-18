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

	empty, err := applyWatchOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty.topics != nil || empty.eventTypes != nil {
		t.Fatalf("zero options %+v", empty)
	}
	if _, err := applyWatchOptions([]WatchOption{nil, WithTopics("bad topic")}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeWatchTopics_SkipsEmpty(t *testing.T) {
	got, err := normalizeWatchTopics([]string{"", "  ", "custom."})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("%q", got)
	}
}

func TestNormalizeWatchTypes_EmptyAndDup(t *testing.T) {
	if got := normalizeWatchTypes(nil); got != nil {
		t.Fatalf("%q", got)
	}
	got := normalizeWatchTypes([]string{"", " member.join ", "member.join"})
	if len(got) != 1 || got[0] != "member.join" {
		t.Fatalf("%q", got)
	}
}

func TestValidWatchTopic(t *testing.T) {
	if err := validWatchTopic(""); err == nil {
		t.Fatal("empty")
	}
	if err := validWatchTopic(string(make([]byte, maxWatchTopicLen+1))); err == nil {
		t.Fatal("too long")
	}
}
