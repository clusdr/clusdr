package operator

import (
	"fmt"
	"strconv"
	"strings"
)

func dropSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

func memberAddr(p PodAddr) string {
	if p.HostIP == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", p.HostIP, grpcPort)
}

func joinAddr(p PodAddr) string {
	if p.Addr != "" {
		return p.Addr
	}
	return memberAddr(p)
}

func podOrdinal(name string) int {
	i := strings.LastIndex(name, "-")
	if i < 0 || i+1 >= len(name) {
		return -1
	}
	n, err := strconv.Atoi(name[i+1:])
	if err != nil {
		return -1
	}
	return n
}

// alreadyMember is true when Raft already has this daemon (alive or dead).
// Restart with intact data.dir must not call join.
func alreadyMember(st Status, p PodAddr) bool {
	addr := joinAddr(p)
	for _, m := range st.Members {
		if p.NodeName != "" && m.ID == p.NodeName {
			return true
		}
		if addr != "" && m.Address != "" && (m.Address == addr || m.Address == p.HostIP) {
			return true
		}
	}
	return false
}

func memberListed(st Status, id string) bool {
	for _, m := range st.Members {
		if m.ID == id {
			return true
		}
	}
	return false
}
