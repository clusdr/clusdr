package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// Kube implements Platform with client-go.
type Kube struct {
	Core kubernetes.Interface
	Dyn  dynamic.Interface
}

func (k Kube) ReadyNodes(ctx context.Context) ([]string, error) {
	list, err := k.Core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range list.Items {
		if n.Spec.Unschedulable {
			continue
		}
		ready := false
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		if ready {
			out = append(out, n.Name)
		}
	}
	return out, nil
}

func (k Kube) ClaimSeed(ctx context.Context, c Cluster, node string) error {
	u, err := k.Dyn.Resource(GVR).Namespace(c.Namespace).Get(ctx, c.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if c.ResourceVersion != "" {
		u.SetResourceVersion(c.ResourceVersion)
	}
	status, _, _ := unstructured.NestedMap(u.Object, "status")
	if status == nil {
		status = map[string]any{}
	}
	status["seedNodeName"] = node
	if err := unstructured.SetNestedMap(u.Object, status, "status"); err != nil {
		return err
	}
	_, err = k.Dyn.Resource(GVR).Namespace(c.Namespace).UpdateStatus(ctx, u, metav1.UpdateOptions{})
	return err
}

func (k Kube) ClusterCount(ctx context.Context) (int, error) {
	list, err := k.Dyn.Resource(GVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(list.Items), nil
}

func (k Kube) EnsurePrepare(ctx context.Context, spec Spec, c Cluster) error {
	want := PrepareDaemonSet(c.Namespace, c.Name, c.UID, spec)
	_, err := k.Core.AppsV1().DaemonSets(c.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.AppsV1().DaemonSets(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	}
	return err
}

func (k Kube) PrepareReady(ctx context.Context, c Cluster) (bool, error) {
	ds, err := k.Core.AppsV1().DaemonSets(c.Namespace).Get(ctx, prepareName(c.Name), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	want := ds.Status.DesiredNumberScheduled
	return want > 0 && ds.Status.NumberReady >= want, nil
}

func (k Kube) EnsureJob(ctx context.Context, spec Spec, c Cluster) error {
	want := SeedInitJob(c.Namespace, c.Name, c.UID, spec)
	_, err := k.Core.BatchV1().Jobs(c.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.BatchV1().Jobs(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	}
	return err
}

func jobName(c Cluster) string {
	if c.Spec.topology() == topoSidecar {
		return sidecarBootJob(c.Name)
	}
	return initJobName(c.Name)
}

func (k Kube) JobComplete(ctx context.Context, c Cluster) (bool, error) {
	j, err := k.Core.BatchV1().Jobs(c.Namespace).Get(ctx, jobName(c), metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return j.Status.Succeeded > 0, nil
}

func (k Kube) JobLogs(ctx context.Context, c Cluster) (string, error) {
	pods, err := k.Core.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "job-name=" + jobName(c),
	})
	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no init pods")
	}
	req := k.Core.CoreV1().Pods(c.Namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{Container: "init"})
	rc, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (k Kube) Token(ctx context.Context, c Cluster) (string, bool, error) {
	s, err := k.Core.CoreV1().Secrets(c.Namespace).Get(ctx, tokenSecretName(c.Name), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	b := s.Data["token"]
	if len(b) == 0 {
		return "", false, nil
	}
	return string(b), true, nil
}

func (k Kube) PutToken(ctx context.Context, c Cluster, token string) error {
	name := tokenSecretName(c.Name)
	existing, err := k.Core.CoreV1().Secrets(c.Namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.CoreV1().Secrets(c.Namespace).Create(ctx, TokenSecret(c.Namespace, c.Name, c.UID, token), metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	existing.StringData = map[string]string{"token": token}
	_, err = k.Core.CoreV1().Secrets(c.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func (k Kube) EnsureSeed(ctx context.Context, spec Spec, c Cluster) error {
	want := SeedDeploy(c.Namespace, c.Name, c.UID, spec)
	_, err := k.Core.AppsV1().Deployments(c.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.AppsV1().Deployments(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	}
	return err
}

func (k Kube) SeedReadyAddr(ctx context.Context, c Cluster) (string, error) {
	if c.Spec.topology() == topoSidecar {
		name := stsName(c.Name) + "-0"
		p, err := k.Core.CoreV1().Pods(c.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		if !podReady(*p) {
			return "", fmt.Errorf("sidecar-0 not ready")
		}
		return sidecarPodDNS(name, c), nil
	}
	pods, err := k.Core.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector(c.Name, compSeed),
	})
	if err != nil {
		return "", err
	}
	for _, p := range pods.Items {
		if p.Status.HostIP == "" {
			continue
		}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name == "clusdr" && cs.Ready {
				return fmt.Sprintf("%s:%d", p.Status.HostIP, grpcPort), nil
			}
		}
	}
	return "", fmt.Errorf("seed pod not ready")
}

func (k Kube) EnsureDaemonSet(ctx context.Context, spec Spec, c Cluster) error {
	want := MemberDaemonSet(c.Namespace, c.Name, c.UID, spec)
	_, err := k.Core.AppsV1().DaemonSets(c.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.AppsV1().DaemonSets(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	}
	return err
}

func (k Kube) MemberPods(ctx context.Context, c Cluster) ([]PodAddr, error) {
	comp := compMember
	if c.Spec.topology() == topoSidecar {
		comp = compSidecar
	}
	pods, err := k.Core.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector(c.Name, comp),
	})
	if err != nil {
		return nil, err
	}
	out := make([]PodAddr, 0, len(pods.Items))
	for _, p := range pods.Items {
		pa := PodAddr{Name: p.Name, HostIP: p.Status.HostIP, NodeName: p.Spec.NodeName, Ready: podReady(p)}
		if c.Spec.topology() == topoSidecar {
			pa.NodeName = p.Name
			pa.Addr = sidecarPodDNS(p.Name, c)
		} else {
			pa.Addr = memberAddr(pa)
		}
		out = append(out, pa)
	}
	return out, nil
}

func podReady(p corev1.Pod) bool {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Name == "clusdr" {
			return cs.Ready
		}
	}
	return false
}

func (k Kube) PatchStatus(ctx context.Context, c Cluster, st Status) error {
	body, err := json.Marshal(map[string]any{"status": statusMap(st)})
	if err != nil {
		return err
	}
	_, err = k.Dyn.Resource(GVR).Namespace(c.Namespace).Patch(
		ctx, c.Name, types.MergePatchType, body, metav1.PatchOptions{}, "status",
	)
	return err
}

func (k Kube) EnsurePVC(ctx context.Context, spec Spec, c Cluster) error {
	_ = spec
	want := SidecarPVC0(c.Namespace, c.Name, c.UID)
	_, err := k.Core.CoreV1().PersistentVolumeClaims(c.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.CoreV1().PersistentVolumeClaims(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	}
	return err
}

func (k Kube) EnsureBootstrapJob(ctx context.Context, spec Spec, c Cluster) error {
	want := SidecarBootstrapJob(c.Namespace, c.Name, c.UID, spec)
	_, err := k.Core.BatchV1().Jobs(c.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.BatchV1().Jobs(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	}
	return err
}

func (k Kube) BootstrapReady(ctx context.Context, c Cluster) (bool, error) {
	pods, err := k.Core.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "job-name=" + sidecarBootJob(c.Name),
	})
	if err != nil {
		return false, err
	}
	for _, p := range pods.Items {
		if podReady(p) {
			return true, nil
		}
	}
	return false, nil
}

func (k Kube) JobExists(ctx context.Context, c Cluster) (bool, error) {
	_, err := k.Core.BatchV1().Jobs(c.Namespace).Get(ctx, sidecarBootJob(c.Name), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (k Kube) DeleteJob(ctx context.Context, c Cluster) error {
	fg := metav1.DeletePropagationForeground
	err := k.Core.BatchV1().Jobs(c.Namespace).Delete(ctx, sidecarBootJob(c.Name), metav1.DeleteOptions{
		PropagationPolicy: &fg,
	})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (k Kube) EnsureHeadless(ctx context.Context, spec Spec, c Cluster) error {
	_ = spec
	want := HeadlessService(c.Namespace, c.Name, c.UID)
	_, err := k.Core.CoreV1().Services(c.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.CoreV1().Services(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	}
	return err
}

func (k Kube) EnsureStatefulSet(ctx context.Context, spec Spec, c Cluster) error {
	want := MemberStatefulSet(c.Namespace, c.Name, c.UID, spec)
	_, err := k.Core.AppsV1().StatefulSets(c.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k.Core.AppsV1().StatefulSets(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	}
	return err
}
