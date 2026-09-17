package operator

import (
	"testing"
)

func TestAlreadyMember(t *testing.T) {
	t.Parallel()
	st := Status{Members: []MemberStatus{
		{ID: "worker-a", Address: "10.0.0.2:7947", Status: "dead"},
	}}
	if !alreadyMember(st, PodAddr{HostIP: "10.0.0.2", NodeName: "other", Ready: true}) {
		t.Fatal("address match")
	}
	if !alreadyMember(st, PodAddr{HostIP: "10.0.0.9", NodeName: "worker-a", Ready: true}) {
		t.Fatal("nodeName match")
	}
	if alreadyMember(st, PodAddr{HostIP: "10.0.0.3", NodeName: "worker-b", Ready: true}) {
		t.Fatal("new node")
	}
	empty := Status{Members: []MemberStatus{{ID: "seed", Address: "", Status: "alive"}}}
	if alreadyMember(empty, PodAddr{Name: "clusdr-1", NodeName: "clusdr-1", Addr: "clusdr-1.svc:7947", Ready: true}) {
		t.Fatal("empty address must not match")
	}
}

func TestPodOrdinal(t *testing.T) {
	t.Parallel()
	if podOrdinal("clusdr-0") != 0 || podOrdinal("clusdr-2") != 2 {
		t.Fatal("ordinal")
	}
	if podOrdinal("clusdr") != -1 {
		t.Fatal("no ordinal")
	}
}
