package controllers

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

const (
	timeout  = 20 * time.Second
	interval = 100 * time.Millisecond
)

func TestBoxProvisionsNamespaceAndRBAC(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	id := string(uuid.NewUUID())

	box := &agentv1.AgentBox{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(id)},
		Spec: agentv1.AgentBoxSpec{
			AgentSessionID: id,
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get"}},
			},
		},
	}
	g.Expect(k8sClient.Create(ctx, box)).To(Succeed())

	g.Eventually(func() agentv1.Phase {
		_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(box), box)
		return box.Status.Phase
	}, timeout, interval).Should(Equal(agentv1.PhaseReady))
	g.Expect(box.Status.Namespace).To(Equal("box-" + id))

	var ns corev1.Namespace
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "box-" + id}, &ns)).To(Succeed())
	g.Expect(ns.Labels).To(HaveKeyWithValue(agentv1.SessionIDLabel, id))
	g.Expect(ns.OwnerReferences).To(HaveLen(1))
	g.Expect(ns.OwnerReferences[0].UID).To(Equal(box.UID))

	var role rbacv1.Role
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "box-" + id, Name: BoxRole}, &role)).To(Succeed())
	g.Expect(role.Rules).To(HaveLen(1))
	var binding rbacv1.RoleBinding
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "box-" + id, Name: BoxRole}, &binding)).To(Succeed())
	g.Expect(binding.Subjects[0].Name).To(Equal(BoxServiceAccount))

	// Deleting the box deletes its namespace and releases the finalizer.
	g.Expect(k8sClient.Delete(ctx, box)).To(Succeed())
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(box), box)
		return client.IgnoreNotFound(err) == nil && err != nil
	}, timeout, interval).Should(BeTrue())
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "box-" + id}, &ns)).To(Succeed())
	g.Expect(ns.DeletionTimestamp).NotTo(BeNil())
}

func TestWorkspaceAndWorkerWaitForBoxThenProvision(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	id := string(uuid.NewUUID())
	ns := "box-" + id

	// Children created before their box (and namespace) exist is not possible,
	// so create the box first and confirm the children reach Ready in order.
	box := &agentv1.AgentBox{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(id)},
		Spec:       agentv1.AgentBoxSpec{AgentSessionID: id},
	}
	g.Expect(k8sClient.Create(ctx, box)).To(Succeed())
	g.Eventually(func() string {
		_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(box), box)
		return box.Status.Namespace
	}, timeout, interval).Should(Equal(ns))

	worker := &agentv1.AgentWorker{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.WorkerName(id), Namespace: ns},
		Spec:       agentv1.AgentWorkerSpec{AgentSessionID: id, Image: "busybox"},
	}
	g.Expect(k8sClient.Create(ctx, worker)).To(Succeed())

	// No workspace yet: the worker must wait and create no pod.
	g.Eventually(func() string {
		_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)
		for _, c := range worker.Status.Conditions {
			return c.Reason
		}
		return ""
	}, timeout, interval).Should(Equal("WaitingForWorkspace"))
	var pod corev1.Pod
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkerName(id)}, &pod)
	g.Expect(client.IgnoreNotFound(err)).To(Succeed())
	g.Expect(err).To(HaveOccurred())

	ws := &agentv1.AgentWorkspace{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.WorkspaceName(id), Namespace: ns},
		Spec:       agentv1.AgentWorkspaceSpec{AgentSessionID: id, Size: resource.MustParse("1Gi")},
	}
	g.Expect(k8sClient.Create(ctx, ws)).To(Succeed())
	g.Eventually(func() agentv1.Phase {
		_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(ws), ws)
		return ws.Status.Phase
	}, timeout, interval).Should(Equal(agentv1.PhaseReady))
	g.Expect(ws.OwnerReferences).To(HaveLen(1))
	g.Expect(ws.OwnerReferences[0].UID).To(Equal(box.UID))

	var pvc corev1.PersistentVolumeClaim
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkspaceName(id)}, &pvc)).To(Succeed())
	g.Expect(pvc.Labels).To(HaveKeyWithValue(agentv1.SessionIDLabel, id))
	g.Expect(pvc.Spec.AccessModes).To(ConsistOf(corev1.ReadWriteOnce))

	// Workspace ready: the worker pod appears and mounts the workspace claim.
	g.Eventually(func() error {
		return k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkerName(id)}, &pod)
	}, timeout, interval).Should(Succeed())
	g.Expect(pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).To(Equal("workspace-" + id))
	g.Expect(pod.Spec.ServiceAccountName).To(Equal(BoxServiceAccount))
	g.Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/workspace"))
	g.Expect(pod.Labels).To(HaveKeyWithValue(agentv1.SessionIDLabel, id))

	g.Eventually(func() string {
		_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(worker), worker)
		return worker.Status.PodName
	}, timeout, interval).Should(Equal("worker-" + id))
	g.Expect(worker.OwnerReferences).To(HaveLen(1))
	g.Expect(worker.OwnerReferences[0].UID).To(Equal(box.UID))
}

func TestSessionIDIsImmutable(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	id := string(uuid.NewUUID())
	box := &agentv1.AgentBox{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(id)},
		Spec:       agentv1.AgentBoxSpec{AgentSessionID: id},
	}
	g.Expect(k8sClient.Create(ctx, box)).To(Succeed())
	box.Spec.AgentSessionID = string(uuid.NewUUID())
	g.Expect(k8sClient.Update(ctx, box)).To(MatchError(ContainSubstring("immutable")))

	bad := &agentv1.AgentBox{
		ObjectMeta: metav1.ObjectMeta{Name: "box-bad"},
		Spec:       agentv1.AgentBoxSpec{AgentSessionID: "not-a-uuid"},
	}
	g.Expect(k8sClient.Create(ctx, bad)).NotTo(Succeed())
}
