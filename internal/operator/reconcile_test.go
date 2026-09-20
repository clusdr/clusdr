package operator

import (
	"context"
	"strings"
	"testing"
)

type fakePlatform struct {
	jobDone        bool
	token          string
	tokenOK        bool
	logs           string
	seedAddr       string
	seedErr        error
	pods           []PodAddr
	status         Status
	ensureJob      int
	ensurePrepare  int
	prepareBlocked bool
	ensureSeed     int
	ensureDS       int
	putToken       string
	ensurePVC      int
	ensureBoot     int
	ensureHeadless int
	ensureSTS      int
	deleteJob      int
	bootReady      bool
	jobExists      bool
	readyNodes     []string
	readyErr       error
	claimedSeed    string
	claims         int
	claimErr       error
	clusterCount   int
}

func (f *fakePlatform) ReadyNodes(context.Context) ([]string, error) {
	if f.readyErr != nil {
		return nil, f.readyErr
	}
	if f.readyNodes != nil {
		return append([]string(nil), f.readyNodes...), nil
	}
	return []string{"kind-worker"}, nil
}
func (f *fakePlatform) ClusterCount(context.Context) (int, error) {
	if f.clusterCount > 0 {
		return f.clusterCount, nil
	}
	return 1, nil
}
func (f *fakePlatform) ClaimSeed(_ context.Context, _ Cluster, node string) error {
	if f.claimErr != nil {
		return f.claimErr
	}
	f.claims++
	f.claimedSeed = node
	return nil
}
func (f *fakePlatform) EnsurePrepare(context.Context, Spec, Cluster) error {
	f.ensurePrepare++
	return nil
}
func (f *fakePlatform) PrepareReady(context.Context, Cluster) (bool, error) {
	return !f.prepareBlocked, nil
}
func (f *fakePlatform) EnsureJob(context.Context, Spec, Cluster) error {
	f.ensureJob++
	return nil
}
func (f *fakePlatform) JobComplete(context.Context, Cluster) (bool, error) {
	return f.jobDone, nil
}
func (f *fakePlatform) JobLogs(context.Context, Cluster) (string, error) {
	return f.logs, nil
}
func (f *fakePlatform) Token(context.Context, Cluster) (string, bool, error) {
	return f.token, f.tokenOK, nil
}
func (f *fakePlatform) PutToken(_ context.Context, _ Cluster, token string) error {
	f.putToken = token
	f.token = token
	f.tokenOK = true
	return nil
}
func (f *fakePlatform) EnsureSeed(context.Context, Spec, Cluster) error {
	f.ensureSeed++
	return nil
}
func (f *fakePlatform) SeedReadyAddr(context.Context, Cluster) (string, error) {
	return f.seedAddr, f.seedErr
}
func (f *fakePlatform) EnsureDaemonSet(context.Context, Spec, Cluster) error {
	f.ensureDS++
	return nil
}
func (f *fakePlatform) MemberPods(context.Context, Cluster) ([]PodAddr, error) {
	return f.pods, nil
}
func (f *fakePlatform) PatchStatus(_ context.Context, _ Cluster, st Status) error {
	f.status = st
	return nil
}
func (f *fakePlatform) EnsurePVC(context.Context, Spec, Cluster) error {
	f.ensurePVC++
	return nil
}
func (f *fakePlatform) EnsureBootstrapJob(context.Context, Spec, Cluster) error {
	f.ensureBoot++
	return nil
}
func (f *fakePlatform) BootstrapReady(context.Context, Cluster) (bool, error) {
	return f.bootReady, nil
}
func (f *fakePlatform) JobExists(context.Context, Cluster) (bool, error) {
	return f.jobExists, nil
}
func (f *fakePlatform) DeleteJob(context.Context, Cluster) error {
	f.deleteJob++
	f.jobExists = false
	return nil
}
func (f *fakePlatform) EnsureHeadless(context.Context, Spec, Cluster) error {
	f.ensureHeadless++
	return nil
}
func (f *fakePlatform) EnsureStatefulSet(context.Context, Spec, Cluster) error {
	f.ensureSTS++
	return nil
}

type joinCall struct {
	local, seed, token string
	observer           bool
}

type fakeJoiner struct {
	calls  []joinCall
	leaves []string
	snap   Status
}

func (j *fakeJoiner) Join(_ context.Context, local, seed, token string, observer bool) error {
	j.calls = append(j.calls, joinCall{local: local, seed: seed, token: token, observer: observer})
	return nil
}
func (j *fakeJoiner) Leave(_ context.Context, _, nodeID string) error {
	j.leaves = append(j.leaves, nodeID)
	kept := make([]MemberStatus, 0, len(j.snap.Members))
	for _, m := range j.snap.Members {
		if m.ID != nodeID {
			kept = append(kept, m)
		}
	}
	j.snap.Members = kept
	return nil
}
func (j *fakeJoiner) Snapshot(context.Context, string) (Status, error) {
	return j.snap, nil
}

func testCluster() Cluster {
	return Cluster{
		Namespace:  "clusdr",
		Name:       "clusdr",
		UID:        "uid-1",
		Generation: 2,
		Spec:       Spec{Topology: topoDaemon, VoterCount: 3},
	}
}

func TestReconcileSidecarWaitsForBootstrap(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	c := testCluster()
	c.Spec.Topology = topoSidecar
	if err := r.Reconcile(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if kube.ensurePVC != 1 || kube.ensureBoot != 1 {
		t.Fatalf("pvc=%d boot=%d", kube.ensurePVC, kube.ensureBoot)
	}
	if kube.ensureDS != 0 || kube.ensureSeed != 0 {
		t.Fatal("sidecar must not create DaemonSet seed")
	}
	if kube.status.Phase != "Pending" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
}

func TestReconcileEvenVotersError(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	c := testCluster()
	c.Spec.VoterCount = 4
	if err := r.Reconcile(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if kube.status.Phase != "Error" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
	if kube.ensureJob != 0 || kube.ensurePrepare != 0 {
		t.Fatal("bad spec must not create Jobs")
	}
}

func TestReconcileWaitsForPrepare(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{prepareBlocked: true}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.ensurePrepare != 1 {
		t.Fatalf("ensurePrepare=%d", kube.ensurePrepare)
	}
	if kube.ensureJob != 0 {
		t.Fatal("must not init before hostPath is prepared")
	}
	if kube.status.Phase != "Pending" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
}

func TestReconcileWaitsForInit(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{jobDone: false}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.status.Phase != "Pending" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
	if kube.ensureSeed != 0 {
		t.Fatal("must not start seed before init completes")
	}
}

func TestReconcileTokenSecretAndObservers(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		jobDone:  true,
		tokenOK:  true,
		token:    "tok-1",
		seedAddr: "10.0.0.1:7947",
		pods: []PodAddr{
			{Name: "clusdr-b", HostIP: "10.0.0.3", Ready: true},
			{Name: "clusdr-a", HostIP: "10.0.0.2", Ready: true},
			{Name: "clusdr-c", HostIP: "10.0.0.4", Ready: true},
		},
	}
	j := &fakeJoiner{snap: Status{
		ClusterID: "cid",
		Leader:    "seed",
		Members:   []MemberStatus{{ID: "seed", Status: "alive", Role: "leader"}},
	}}
	r := &Reconciler{Kube: kube, Joiner: j}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.ensureJob != 1 || kube.ensureSeed != 1 || kube.ensureDS != 1 {
		t.Fatalf("ensure job=%d seed=%d ds=%d", kube.ensureJob, kube.ensureSeed, kube.ensureDS)
	}
	if len(j.calls) != 3 {
		t.Fatalf("joins %d", len(j.calls))
	}
	if j.calls[0].local != "10.0.0.2:7947" || j.calls[0].observer {
		t.Fatalf("ds0 %+v", j.calls[0])
	}
	if j.calls[1].observer {
		t.Fatalf("ds1 should vote %+v", j.calls[1])
	}
	if !j.calls[2].observer {
		t.Fatalf("ds2 should observe %+v", j.calls[2])
	}
	if j.calls[0].token != "tok-1" || j.calls[0].seed != "10.0.0.1:7947" {
		t.Fatalf("join seed/token %+v", j.calls[0])
	}
	if kube.status.Phase != "Ready" || kube.status.ClusterID != "cid" {
		t.Fatalf("status %+v", kube.status)
	}
	if kube.status.ObservedGeneration != 2 {
		t.Fatalf("observedGeneration %d", kube.status.ObservedGeneration)
	}
	if len(j.leaves) != 0 {
		t.Fatalf("leave on form: %v", j.leaves)
	}
}

func TestReconcilePersistsTokenFromLogs(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		jobDone:  true,
		logs:     "  join token      : from-logs\n",
		seedAddr: "10.0.0.1:7947",
	}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{snap: Status{ClusterID: "cid"}}}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.putToken != "from-logs" {
		t.Fatalf("putToken %q", kube.putToken)
	}
	if kube.status.Phase != "Ready" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
}

func TestReconcileBounceDoesNotJoinOrLeave(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		jobDone:  true,
		tokenOK:  true,
		token:    "tok-1",
		seedAddr: "10.0.0.1:7947",
		pods: []PodAddr{
			{Name: "clusdr-a", HostIP: "10.0.0.2", NodeName: "worker-a", Ready: false},
		},
	}
	dead := MemberStatus{ID: "worker-a", Address: "10.0.0.2:7947", Status: "dead", Role: "voter"}
	j := &fakeJoiner{snap: Status{
		ClusterID: "cid",
		Leader:    "seed",
		Members: []MemberStatus{
			{ID: "seed", Address: "10.0.0.1:7947", Status: "alive", Role: "leader"},
			dead,
		},
	}}
	r := &Reconciler{Kube: kube, Joiner: j}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if len(j.calls) != 0 {
		t.Fatalf("restart must not join: %+v", j.calls)
	}
	if len(j.leaves) != 0 {
		t.Fatalf("missing pod must not leave: %v", j.leaves)
	}
	if kube.status.Phase != "Ready" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
	found := false
	for _, m := range kube.status.Members {
		if m.ID == "worker-a" && m.Status == "dead" {
			found = true
		}
	}
	if !found {
		t.Fatalf("dead id dropped from status: %+v", kube.status.Members)
	}
}

func TestReconcileAlreadyMemberSkipsJoin(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		jobDone:  true,
		tokenOK:  true,
		token:    "tok-1",
		seedAddr: "10.0.0.1:7947",
		pods: []PodAddr{
			{Name: "clusdr-a", HostIP: "10.0.0.2", NodeName: "worker-a", Ready: true},
		},
	}
	j := &fakeJoiner{snap: Status{
		ClusterID: "cid",
		Members: []MemberStatus{
			{ID: "seed", Address: "10.0.0.1:7947", Status: "alive", Role: "leader"},
			{ID: "worker-a", Address: "10.0.0.2:7947", Status: "alive", Role: "voter"},
		},
	}}
	r := &Reconciler{Kube: kube, Joiner: j}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if len(j.calls) != 0 {
		t.Fatalf("already a member must not join: %+v", j.calls)
	}
	if len(j.leaves) != 0 {
		t.Fatalf("leave: %v", j.leaves)
	}
}

func TestReconcileSpecLeaveDropsMember(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		jobDone:  true,
		tokenOK:  true,
		token:    "tok-1",
		seedAddr: "10.0.0.1:7947",
		pods: []PodAddr{
			{Name: "clusdr-a", HostIP: "10.0.0.2", NodeName: "worker-a", Ready: true},
		},
	}
	j := &fakeJoiner{snap: Status{
		ClusterID: "cid",
		Members: []MemberStatus{
			{ID: "seed", Address: "10.0.0.1:7947", Status: "alive", Role: "leader"},
			{ID: "worker-a", Address: "10.0.0.2:7947", Status: "dead", Role: "voter"},
		},
	}}
	c := testCluster()
	c.Spec.Leave = []string{"worker-a"}
	r := &Reconciler{Kube: kube, Joiner: j}
	if err := r.Reconcile(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if len(j.leaves) != 1 || j.leaves[0] != "worker-a" {
		t.Fatalf("leave %v", j.leaves)
	}
	if len(j.calls) != 0 {
		t.Fatalf("must not re-join a left id: %+v", j.calls)
	}
	for _, m := range kube.status.Members {
		if m.ID == "worker-a" {
			t.Fatalf("left id still in status: %+v", kube.status.Members)
		}
	}
}

func TestReconcileMissingPodDoesNotLeave(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		jobDone:  true,
		tokenOK:  true,
		token:    "tok-1",
		seedAddr: "10.0.0.1:7947",
	}
	j := &fakeJoiner{snap: Status{
		ClusterID: "cid",
		Members: []MemberStatus{
			{ID: "seed", Status: "alive", Role: "leader"},
			{ID: "worker-a", Address: "10.0.0.2:7947", Status: "dead", Role: "voter"},
			{ID: "worker-b", Address: "10.0.0.3:7947", Status: "alive", Role: "voter"},
		},
	}}
	r := &Reconciler{Kube: kube, Joiner: j}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if len(j.leaves) != 0 {
		t.Fatalf("empty MemberPods must not leave: %v", j.leaves)
	}
	if len(kube.status.Members) != 3 {
		t.Fatalf("status members %d", len(kube.status.Members))
	}
}

func sidecarCluster() Cluster {
	c := testCluster()
	c.Spec.Topology = topoSidecar
	return c
}

func TestReconcileSidecarPersistsTokenAndDeletesJob(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		bootReady: true,
		logs:      "  join token      : side-tok\n",
		jobExists: true,
	}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), sidecarCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.putToken != "side-tok" {
		t.Fatalf("putToken %q", kube.putToken)
	}
	if kube.deleteJob != 1 {
		t.Fatalf("deleteJob %d", kube.deleteJob)
	}
	if kube.ensureSTS != 0 {
		t.Fatal("must delete bootstrap Job before STS")
	}
	if kube.status.Phase != "Pending" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
}

func TestReconcileSidecarJoinsOrdinalsNotZero(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		tokenOK:  true,
		token:    "tok-1",
		seedAddr: "clusdr-0.clusdr.clusdr.svc.cluster.local:7947",
		pods: []PodAddr{
			{Name: "clusdr-0", NodeName: "clusdr-0", Addr: "clusdr-0.clusdr.clusdr.svc.cluster.local:7947", Ready: true},
			{Name: "clusdr-2", NodeName: "clusdr-2", Addr: "clusdr-2.clusdr.clusdr.svc.cluster.local:7947", Ready: true},
			{Name: "clusdr-1", NodeName: "clusdr-1", Addr: "clusdr-1.clusdr.clusdr.svc.cluster.local:7947", Ready: true},
		},
	}
	j := &fakeJoiner{snap: Status{
		ClusterID: "cid",
		Members:   []MemberStatus{{ID: "clusdr-0", Status: "alive", Role: "leader"}},
	}}
	r := &Reconciler{Kube: kube, Joiner: j}
	if err := r.Reconcile(context.Background(), sidecarCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.ensureHeadless != 1 || kube.ensureSTS != 1 {
		t.Fatalf("headless=%d sts=%d", kube.ensureHeadless, kube.ensureSTS)
	}
	if kube.ensureDS != 0 {
		t.Fatal("sidecar must not create a DaemonSet")
	}
	if len(j.calls) != 2 {
		t.Fatalf("joins %d %+v", len(j.calls), j.calls)
	}
	if j.calls[0].local != "clusdr-2.clusdr.clusdr.svc.cluster.local:7947" && j.calls[0].local != "clusdr-1.clusdr.clusdr.svc.cluster.local:7947" {
		t.Fatalf("join local %q", j.calls[0].local)
	}
	for _, call := range j.calls {
		if call.observer {
			t.Fatalf("3 voters: joiners should vote %+v", call)
		}
		if strings.Contains(call.local, "clusdr-0.") {
			t.Fatal("must not join ordinal 0")
		}
	}
	if kube.status.Phase != "Ready" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
}
