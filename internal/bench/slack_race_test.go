//go:build race

package bench_test

// raceSlack widens election/lock p95 under -race. The test still fails
// if a leader is not elected.
const raceSlack = 4
