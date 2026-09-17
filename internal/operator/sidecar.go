package operator

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	pauseImg = "registry.k8s.io/pause:3.10"
	pvcQty   = "1Gi"
)

func sidecarFSGroup() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsUser:  ptr.To(runAs),
		RunAsGroup: ptr.To(runAs),
		FSGroup:    ptr.To(runAs),
	}
}

func sidecarDNSEnv(ns, svc, dir string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
		}},
		{Name: "CLUSDR_NODE_ID", Value: "$(POD_NAME)"},
		{Name: "CLUSDR_DATA_DIR", Value: dir},
		{Name: "CLUSDR_CONTROL_SOCKET", Value: dir + "/clusdr.sock"},
		{Name: "CLUSDR_GRPC_ADDR", Value: "0.0.0.0:7947"},
		{Name: "CLUSDR_NODE_ADDR", Value: "$(POD_NAME)." + svc + "." + ns + ".svc.cluster.local:7947"},
		{Name: "CLUSDR_RAFT_ADDR", Value: "$(POD_NAME)." + svc + "." + ns + ".svc.cluster.local:7946"},
		{Name: "CLUSDR_LOG_FORMAT", Value: "json"},
		{Name: "CLUSDR_LOG_LEVEL", Value: "info"},
	}
}

func pvcSpec() corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(pvcQty),
			},
		},
	}
}

// SidecarPVC0 is ordinal-0 identity (not emptyDir). STS adopts this name.
func SidecarPVC0(ns, crName, uid string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            sidecarPVC0(crName),
			Namespace:       ns,
			Labels:          labels(crName, compSidecar),
			OwnerReferences: owner(crName, uid),
		},
		Spec: pvcSpec(),
	}
}

// SidecarBootstrapJob inits and --bootstraps PVC-0. Delete it before the STS (RWO).
func SidecarBootstrapJob(ns, crName, uid string, spec Spec) *batchv1.Job {
	dir := spec.dataDir()
	svc := headlessName(crName)
	seedID := stsName(crName) + "-0"
	live, ready := tcpProbes()
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            sidecarBootJob(crName),
			Namespace:       ns,
			Labels:          labels(crName, compSidecarBoot),
			OwnerReferences: owner(crName, uid),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(0)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels(crName, compSidecarBoot)},
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					SecurityContext: sidecarFSGroup(),
					InitContainers: []corev1.Container{{
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
					Containers: []corev1.Container{{
						Name:            "clusdr",
						Image:           spec.image(),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args:            []string{"start", "--bootstrap", "--config", dir + "/clusdr.yaml"},
						Env: []corev1.EnvVar{
							{Name: "CLUSDR_NODE_ID", Value: seedID},
							{Name: "CLUSDR_DATA_DIR", Value: dir},
							{Name: "CLUSDR_CONTROL_SOCKET", Value: dir + "/clusdr.sock"},
							{Name: "CLUSDR_GRPC_ADDR", Value: "0.0.0.0:7947"},
							{Name: "CLUSDR_NODE_ADDR", Value: seedID + "." + svc + "." + ns + ".svc.cluster.local:7947"},
							{Name: "CLUSDR_RAFT_ADDR", Value: seedID + "." + svc + "." + ns + ".svc.cluster.local:7946"},
							{Name: "CLUSDR_LOG_FORMAT", Value: "json"},
						},
						VolumeMounts:   []corev1.VolumeMount{{Name: "data", MountPath: dir}},
						Ports:          ports(),
						LivenessProbe:  &live,
						ReadinessProbe: &ready,
					}},
					Volumes: []corev1.Volume{{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: sidecarPVC0(crName),
							},
						},
					}},
				},
			},
		},
	}
}

// HeadlessService is advertised node.addr DNS. Not a ClusterIP for Local().
func HeadlessService(ns, crName, uid string) *corev1.Service {
	sel := labels(crName, compSidecar)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            headlessName(crName),
			Namespace:       ns,
			Labels:          sel,
			OwnerReferences: owner(crName, uid),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                corev1.ClusterIPNone,
			PublishNotReadyAddresses: true,
			Selector:                 sel,
			Ports: []corev1.ServicePort{
				{Name: "grpc", Port: grpcPort, Protocol: corev1.ProtocolTCP},
				{Name: "raft", Port: raftPort, Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

// MemberStatefulSet is the replica-is-the-member exception. PVC, not emptyDir.
// start without --bootstrap. App dials 127.0.0.1. Not a Deployment.
func MemberStatefulSet(ns, crName, uid string, spec Spec) *appsv1.StatefulSet {
	dir := spec.dataDir()
	svc := headlessName(crName)
	sel := labels(crName, compSidecar)
	live, ready := tcpProbes()
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            stsName(crName),
			Namespace:       ns,
			Labels:          sel,
			OwnerReferences: owner(crName, uid),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         svc,
			Replicas:            ptr.To(i32(spec.VoterCount)),
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			Selector:            &metav1.LabelSelector{MatchLabels: sel},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: sel},
				Spec: corev1.PodSpec{
					SecurityContext: sidecarFSGroup(),
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: pauseImg,
							Env: []corev1.EnvVar{
								{Name: "CLUSDR_GRPC_ADDR", Value: "127.0.0.1:7947"},
								{Name: "CLUSDR_DATA_DIR", Value: dir},
							},
							VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: dir, ReadOnly: true}},
						},
						{
							Name:            "clusdr",
							Image:           spec.image(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"start", "--config", dir + "/clusdr.yaml"},
							Env:             sidecarDNSEnv(ns, svc, dir),
							VolumeMounts:    []corev1.VolumeMount{{Name: "data", MountPath: dir}},
							Ports:           ports(),
							LivenessProbe:   &live,
							ReadinessProbe:  &ready,
							Resources:       resources(),
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec:       pvcSpec(),
			}},
		},
	}
}
