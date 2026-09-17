package operator

import (
	"testing"
)

func TestParseJoinToken(t *testing.T) {
	t.Parallel()
	const logs = `cluster initialized

  cluster id      : abc
  join token      : tok-secret-1

  The join token is shown only once. Store it securely.
`
	got, err := ParseJoinToken(logs)
	if err != nil {
		t.Fatal(err)
	}
	if got != "tok-secret-1" {
		t.Fatalf("got %q", got)
	}
	if _, err := ParseJoinToken("already initialized\n"); err == nil {
		t.Fatal("want error")
	}
}

func TestMemberObserver(t *testing.T) {
	t.Parallel()
	// 3 voters: seed + DS0 + DS1 voters; DS2 observer
	if MemberObserver(0, 3) {
		t.Fatal("ds0 should vote")
	}
	if MemberObserver(1, 3) {
		t.Fatal("ds1 should vote")
	}
	if !MemberObserver(2, 3) {
		t.Fatal("ds2 should observe")
	}
}
