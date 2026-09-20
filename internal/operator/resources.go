package operator

import (
	"math"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	appName         = "clusdr"
	partOf          = "clusdr"
	compInit        = "seed-init"
	compSeed        = "seed"
	compMember      = "member"
	compPrepare     = "prepare"
	compSidecar     = "sidecar"
	compSidecarBoot = "sidecar-bootstrap"
	runAs           = int64(65532)
	grpcPort        = 7947
	raftPort        = 7946
	defaultImg      = "durguto/clusdr:0.2.0"
	defaultPrepare  = "busybox:1.37.0"
	defaultDir      = "/var/lib/clusdr"
	topoDaemon      = "DaemonSet"
	topoSidecar     = "Sidecar"
)

// GVR is clusdrclusters.clusdr.io/v1alpha1.
var GVR = schema.GroupVersionResource{
	Group:    "clusdr.io",
	Version:  "v1alpha1",
	Resource: "clusdrclusters",
}

// Spec is the CR desired host topology.
type Spec struct {
	Topology     string
	VoterCount   int
	Image        string
	DataDir      string
	SeedNodeName string
	Leave        []string
}

func (s Spec) image() string {
	if s.Image != "" {
		return s.Image
	}
	return defaultImg
}

func (s Spec) dataDir() string {
	if s.DataDir != "" {
		return s.DataDir
	}
	return defaultDir
}

func (s Spec) topology() string {
	if s.Topology == "" {
		return topoDaemon
	}
	return s.Topology
}

func i32(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

func initJobName(cr string) string     { return cr + "-seed-init" }
func seedName(cr string) string        { return cr + "-seed" }
func prepareName(cr string) string     { return cr + "-prepare" }
func tokenSecretName(cr string) string { return cr + "-join-token" }
func daemonSetName(cr string) string   { return cr }
func stsName(cr string) string         { return cr }
func headlessName(cr string) string    { return cr }
func sidecarBootJob(cr string) string  { return cr + "-sidecar-bootstrap" }
func sidecarPVC0(cr string) string     { return "data-" + cr + "-0" }

func labels(cr, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      appName,
		"app.kubernetes.io/instance":  cr,
		"app.kubernetes.io/component": component,
		"app.kubernetes.io/part-of":   partOf,
	}
}

func selector(cr, component string) string {
	return "app.kubernetes.io/instance=" + cr + ",app.kubernetes.io/component=" + component
}

func owner(name, uid string) []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         "clusdr.io/v1alpha1",
		Kind:               "ClusdrCluster",
		Name:               name,
		UID:                types.UID(uid),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}}
}

func controlPlaneToleration() []corev1.Toleration {
	return []corev1.Toleration{{
		Key:      "node-role.kubernetes.io/control-plane",
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoSchedule,
	}}
}

func tcpProbes() (live, ready corev1.Probe) {
	h := corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{
		Port: intstr.FromInt32(grpcPort),
	}}
	live = corev1.Probe{ProbeHandler: h, InitialDelaySeconds: 3, PeriodSeconds: 10, TimeoutSeconds: 5}
	ready = corev1.Probe{ProbeHandler: h, InitialDelaySeconds: 2, PeriodSeconds: 5, TimeoutSeconds: 5}
	return live, ready
}

func hostPathVol(dir string) corev1.Volume {
	t := corev1.HostPathDirectoryOrCreate
	return corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: dir, Type: &t},
		},
	}
}

func daemonEnv(dir string, withNodeID bool) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "NODE_IP", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
		}},
		{Name: "CLUSDR_DATA_DIR", Value: dir},
		{Name: "CLUSDR_CONTROL_SOCKET", Value: dir + "/clusdr.sock"},
		{Name: "CLUSDR_GRPC_ADDR", Value: "0.0.0.0:7947"},
		{Name: "CLUSDR_NODE_ADDR", Value: "$(NODE_IP):7947"},
		{Name: "CLUSDR_RAFT_ADDR", Value: "$(NODE_IP):7946"},
		{Name: "CLUSDR_LOG_FORMAT", Value: "json"},
		{Name: "CLUSDR_LOG_LEVEL", Value: "info"},
	}
	if withNodeID {
		env = append(env, corev1.EnvVar{Name: "CLUSDR_NODE_ID", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
		}})
	}
	return env
}

func resources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("20m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
}

func podSecurity() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsUser:  ptr.To(runAs),
		RunAsGroup: ptr.To(runAs),
	}
}

func ports() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "grpc", ContainerPort: grpcPort, Protocol: corev1.ProtocolTCP},
		{Name: "raft", ContainerPort: raftPort, Protocol: corev1.ProtocolTCP},
	}
}

func nodeName(spec Spec) string {
	return spec.SeedNodeName
}

func prepareResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1m"),
			corev1.ResourceMemory: resource.MustParse("8Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("16Mi"),
		},
	}
}

// prepareContainer chowns hostPath as root. stay=true keeps the DaemonSet
// pod running so kube does not restart it. Distroless /clusdr has no shell.
func prepareContainer(dir string, stay bool) corev1.Container {
	cmd := `mkdir -p "$CLUSDR_DATA_DIR" && chown 65532:65532 "$CLUSDR_DATA_DIR"`
	if stay {
		cmd += " && exec sleep infinity"
	}
	return corev1.Container{
		Name:            "prepare",
		Image:           defaultPrepare,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sh", "-c", cmd},
		Env:             []corev1.EnvVar{{Name: "CLUSDR_DATA_DIR", Value: dir}},
		VolumeMounts:    []corev1.VolumeMount{{Name: "data", MountPath: dir}},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:  ptr.To(int64(0)),
			RunAsGroup: ptr.To(int64(0)),
		},
		Resources: prepareResources(),
	}
}

// SeedInitJob is clusdr init once on the seed node's hostPath.
func SeedInitJob(ns, crName, uid string, spec Spec) *batchv1.Job {
	dir := spec.dataDir()
	j := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            initJobName(crName),
			Namespace:       ns,
			Labels:          labels(crName, compInit),
			OwnerReferences: owner(crName, uid),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels(crName, compInit)},
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					HostNetwork:     true,
					DNSPolicy:       corev1.DNSClusterFirstWithHostNet,
					Tolerations:     controlPlaneToleration(),
					SecurityContext: podSecurity(),
					NodeName:        nodeName(spec),
					InitContainers:  []corev1.Container{prepareContainer(dir, false)},
					Containers: []corev1.Container{{
						Name:            "init",
						Image:           spec.image(),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/clusdr", "init"},
						Args:            []string{"--config", dir + "/clusdr.yaml"},
						Env: []corev1.EnvVar{
							{Name: "CLUSDR_DATA_DIR", Value: dir},
							{Name: "CLUSDR_LOG_FORMAT", Value: "json"},
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: dir}},
					}},
					Volumes: []corev1.Volume{hostPathVol(dir)},
				},
			},
		},
	}
	return j
}

// SeedDeploy is one voter: start --bootstrap. Not applied to joiners.
func SeedDeploy(ns, crName, uid string, spec Spec) *appsv1.Deployment {
	dir := spec.dataDir()
	live, ready := tcpProbes()
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            seedName(crName),
			Namespace:       ns,
			Labels:          labels(crName, compSeed),
			OwnerReferences: owner(crName, uid),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{MatchLabels: labels(crName, compSeed)},
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels(crName, compSeed)},
				Spec: corev1.PodSpec{
					HostNetwork:     true,
					DNSPolicy:       corev1.DNSClusterFirstWithHostNet,
					Tolerations:     controlPlaneToleration(),
					SecurityContext: podSecurity(),
					NodeName:        nodeName(spec),
					InitContainers:  []corev1.Container{prepareContainer(dir, false)},
					Containers: []corev1.Container{{
						Name:            "clusdr",
						Image:           spec.image(),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args:            []string{"start", "--bootstrap", "--config", dir + "/clusdr.yaml"},
						Env:             daemonEnv(dir, false),
						VolumeMounts:    []corev1.VolumeMount{{Name: "data", MountPath: dir}},
						Ports:           ports(),
						LivenessProbe:   &live,
						ReadinessProbe:  &ready,
						Resources:       resources(),
					}},
					Volumes: []corev1.Volume{hostPathVol(dir)},
				},
			},
		},
	}
}

// MemberDaemonSet is start (no bootstrap, no init) on every other node.
func MemberDaemonSet(ns, crName, uid string, spec Spec) *appsv1.DaemonSet {
	dir := spec.dataDir()
	live, ready := tcpProbes()
	sel := labels(crName, compMember)
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            daemonSetName(crName),
			Namespace:       ns,
			Labels:          sel,
			OwnerReferences: owner(crName, uid),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: sel},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: sel},
				Spec: corev1.PodSpec{
					HostNetwork:     true,
					DNSPolicy:       corev1.DNSClusterFirstWithHostNet,
					Tolerations:     controlPlaneToleration(),
					SecurityContext: podSecurity(),
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
								TopologyKey:   corev1.LabelHostname,
								LabelSelector: &metav1.LabelSelector{MatchLabels: labels(crName, compSeed)},
							}},
						},
					},
					InitContainers: []corev1.Container{prepareContainer(dir, false)},
					Containers: []corev1.Container{{
						Name:            "clusdr",
						Image:           spec.image(),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args:            []string{"start"},
						Env:             daemonEnv(dir, true),
						VolumeMounts:    []corev1.VolumeMount{{Name: "data", MountPath: dir}},
						Ports:           ports(),
						LivenessProbe:   &live,
						ReadinessProbe:  &ready,
						Resources:       resources(),
					}},
					Volumes: []corev1.Volume{hostPathVol(dir)},
				},
			},
		},
	}
}

// PrepareDaemonSet chowns dataDir on every node (hostPath ignores fsGroup).
// Runs as root without privileged. Not baked into the distroless daemon image.
func PrepareDaemonSet(ns, crName, uid string, spec Spec) *appsv1.DaemonSet {
	dir := spec.dataDir()
	sel := labels(crName, compPrepare)
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            prepareName(crName),
			Namespace:       ns,
			Labels:          sel,
			OwnerReferences: owner(crName, uid),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: sel},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: sel},
				Spec: corev1.PodSpec{
					Tolerations: controlPlaneToleration(),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.To(int64(0)),
						RunAsGroup: ptr.To(int64(0)),
					},
					Containers: []corev1.Container{prepareContainer(dir, true)},
					Volumes:    []corev1.Volume{hostPathVol(dir)},
				},
			},
		},
	}
}

// TokenSecret holds the join token (not in git).
func TokenSecret(ns, crName, uid, token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tokenSecretName(crName),
			Namespace:       ns,
			Labels:          labels(crName, compSeed),
			OwnerReferences: owner(crName, uid),
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"token": token,
		},
	}
}
