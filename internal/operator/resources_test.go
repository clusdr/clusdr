package operator

import (
	"testing"
)

func TestSeedBootstrapsJoinersDoNot(t *testing.T) {
	t.Parallel()
	spec := Spec{VoterCount: 3, Image: "durguto/clusdr:0.2.0"}
	job := SeedInitJob("clusdr", "demo", "uid", spec)
	if job.Spec.Template.Spec.Containers[0].Command[1] != "init" {
		t.Fatalf("init command %+v", job.Spec.Template.Spec.Containers[0].Command)
	}
	seed := SeedDeploy("clusdr", "demo", "uid", spec)
	args := seed.Spec.Template.Spec.Containers[0].Args
	if !contains(args, "--bootstrap") {
		t.Fatalf("seed args %v", args)
	}
	ds := MemberDaemonSet("clusdr", "demo", "uid", spec)
	dsArgs := ds.Spec.Template.Spec.Containers[0].Args
	if contains(dsArgs, "--bootstrap") || contains(dsArgs, "init") {
		t.Fatalf("joiner args %v", dsArgs)
	}
	if seed.Spec.Template.Spec.SecurityContext.RunAsUser == nil || *seed.Spec.Template.Spec.SecurityContext.RunAsUser != 65532 {
		t.Fatal("uid 65532")
	}
	if ds.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket == nil {
		t.Fatal("tcp probes")
	}
}

func TestPrepareDaemonSetChownsHostPath(t *testing.T) {
	t.Parallel()
	spec := Spec{VoterCount: 3, DataDir: "/var/lib/clusdr"}
	ds := PrepareDaemonSet("clusdr", "demo", "uid", spec)
	if ds.Name != "demo-prepare" {
		t.Fatalf("name %s", ds.Name)
	}
	c := ds.Spec.Template.Spec.Containers[0]
	if c.Image != defaultPrepare {
		t.Fatalf("image %s", c.Image)
	}
	if c.SecurityContext == nil || c.SecurityContext.RunAsUser == nil || *c.SecurityContext.RunAsUser != 0 {
		t.Fatal("prepare runs as root")
	}
	if c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
		t.Fatal("privileged not required")
	}
	if !contains(c.Command, "sh") {
		t.Fatalf("command %v", c.Command)
	}
	job := SeedInitJob("clusdr", "demo", "uid", spec)
	if len(job.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatal("seed init must chown before /clusdr init")
	}
	if job.Spec.Template.Spec.InitContainers[0].Image == spec.image() {
		t.Fatal("prepare is not the distroless daemon")
	}
}

func TestSidecarStatefulSetNotEmptyDir(t *testing.T) {
	t.Parallel()
	spec := Spec{Topology: topoSidecar, VoterCount: 3, Image: "durguto/clusdr:0.2.0"}
	sts := MemberStatefulSet("clusdr", "demo", "uid", spec)
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 3 {
		t.Fatal("replicas = voterCount")
	}
	args := sts.Spec.Template.Spec.Containers[1].Args
	if contains(args, "--bootstrap") || contains(args, "init") {
		t.Fatalf("STS start only %v", args)
	}
	if len(sts.Spec.VolumeClaimTemplates) == 0 {
		t.Fatal("PVC not emptyDir")
	}
	app := sts.Spec.Template.Spec.Containers[0]
	if app.Name != "app" {
		t.Fatal("app container")
	}
	found := false
	for _, e := range app.Env {
		if e.Name == "CLUSDR_GRPC_ADDR" && e.Value == "127.0.0.1:7947" {
			found = true
		}
	}
	if !found {
		t.Fatal("app Local() is 127.0.0.1")
	}
	svc := HeadlessService("clusdr", "demo", "uid")
	if svc.Spec.ClusterIP != "None" {
		t.Fatalf("headless %q", svc.Spec.ClusterIP)
	}
	job := SidecarBootstrapJob("clusdr", "demo", "uid", spec)
	if job.Spec.Template.Spec.Containers[0].Args[0] != "start" || !contains(job.Spec.Template.Spec.Containers[0].Args, "--bootstrap") {
		t.Fatalf("bootstrap %v", job.Spec.Template.Spec.Containers[0].Args)
	}
	if job.Spec.Template.Spec.Volumes[0].EmptyDir != nil {
		t.Fatal("bootstrap PVC not emptyDir")
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
