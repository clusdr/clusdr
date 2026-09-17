package operator

import (
	"fmt"
	"regexp"
	"strings"
)

var joinTokenLine = regexp.MustCompile(`(?m)^\s*join token\s*:\s*(\S+)\s*$`)

// ParseJoinToken reads the one-time token from clusdr init stdout.
func ParseJoinToken(logs string) (string, error) {
	m := joinTokenLine.FindStringSubmatch(logs)
	if len(m) != 2 {
		return "", fmt.Errorf("join token not in init logs")
	}
	tok := strings.TrimSpace(m[1])
	if tok == "" {
		return "", fmt.Errorf("empty join token")
	}
	return tok, nil
}

// MemberObserver reports whether DaemonSet pod index (0-based, sorted) should
// join as an observer. The seed is voter 1; DS pods fill voters until n, then observers.
func MemberObserver(dsIndex, voterCount int) bool {
	if voterCount < 1 {
		return true
	}
	ordinal := dsIndex + 2 // seed + this pod, 1-based membership count
	return ordinal > voterCount
}
