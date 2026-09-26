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

func newBox(id, class string) *agentv1.AgentBox {
	return &agentv1.AgentBox{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(id)},
		Spec: agentv1.AgentBoxSpec{
			AgentSessionID: id,
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get"}},
			},
			Workspace: agentv1.AgentWorkspaceTemplate{Size: resource.MustParse("1Gi"), StorageClassName: &class},
			Worker:    agentv1.AgentWorkerTemplate{Image: "busybox"},
		},
	}
}

func boxPhase(g Gomega, box *agentv1.AgentBox) agentv1.Phase {
	g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(box), box)).To(Succeed())
	return box.Status.Phase
}

func TestBoxProvisionsEverythingInOrder(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	id := string(uuid.NewUUID())
	ns := agentv1.BoxName(id)

	box := newBox(id, waitClass)
	g.Expect(k8sClient.Create(ctx, box)).To(Succeed())
	g.Eventually(func(g Gomega) agentv1.Phase { return boxPhase(g, box) }, timeout, interval).
		Should(Equal(agentv1.PhaseReady))
	g.Expect(box.Status.Namespace).To(Equal(ns))

	var namespace corev1.Namespace
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: ns}, &namespace)).To(Succeed())
	g.Expect(namespace.Labels).To(HaveKeyWithValue(agentv1.SessionIDLabel, id))
	g.Expect(namespace.Labels).To(HaveKeyWithValue(componentLabel, componentBox))
	g.Expect(namespace.Labels).To(HaveKeyWithValue(managedByLabel, managedByValue))
	g.Expect(box.Finalizers).To(ContainElement(agentv1.BoxFinalizer))
	g.Expect(namespace.OwnerReferences).To(HaveLen(1))
	g.Expect(namespace.OwnerReferences[0].UID).To(Equal(box.UID))

	var role rbacv1.Role
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: BoxRole}, &role)).To(Succeed())
	g.Expect(role.Rules).To(HaveLen(1))
	var binding rbacv1.RoleBinding
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: BoxRole}, &binding)).To(Succeed())
	g.Expect(binding.Subjects[0].Name).To(Equal(BoxServiceAccount))

	var ws agentv1.AgentWorkspace
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkspaceName(id)}, &ws)).To(Succeed())
	g.Expect(ws.OwnerReferences).To(HaveLen(1))
	g.Expect(ws.OwnerReferences[0].UID).To(Equal(box.UID))
	g.Expect(ws.Spec.AgentSessionID).To(Equal(id))
	g.Expect(ws.Spec.Size.String()).To(Equal("1Gi"))

	var worker agentv1.AgentWorker
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkerName(id)}, &worker)).To(Succeed())
	g.Expect(worker.OwnerReferences).To(HaveLen(1))
	g.Expect(worker.OwnerReferences[0].UID).To(Equal(box.UID))
	g.Expect(worker.Spec.Image).To(Equal("busybox"))
	g.Expect(worker.Spec.WorkspaceMountPath).To(Equal("/workspace"))

	var pvc corev1.PersistentVolumeClaim
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkspaceName(id)}, &pvc)).To(Succeed())
	g.Expect(pvc.Labels).To(HaveKeyWithValue(agentv1.SessionIDLabel, id))
	g.Expect(pvc.Labels).To(HaveKeyWithValue(componentLabel, componentWorkspace))
	g.Expect(pvc.Spec.AccessModes).To(ConsistOf(corev1.ReadWriteOnce))

	var pod corev1.Pod
	g.Eventually(func() error {
		return k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkerName(id)}, &pod)
	}, timeout, interval).Should(Succeed())
	g.Expect(pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).To(Equal(agentv1.WorkspaceName(id)))
	g.Expect(pod.Spec.ServiceAccountName).To(Equal(BoxServiceAccount))
	g.Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/workspace"))
	g.Expect(pod.Labels).To(HaveKeyWithValue(componentLabel, componentWorker))

	g.Expect(k8sClient.Delete(ctx, box)).To(Succeed())
	g.Eventually(func() bool {
		return client.IgnoreNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(box), box)) == nil &&
			k8sClient.Get(ctx, client.ObjectKeyFromObject(box), box) != nil
	}, timeout, interval).Should(BeTrue())
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: ns}, &namespace)).To(Succeed())
	g.Expect(namespace.DeletionTimestamp).NotTo(BeNil())
}

func TestWorkerWaitsForBoundWorkspace(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	id := string(uuid.NewUUID())
	ns := agentv1.BoxName(id)

	box := newBox(id, immediateClass)
	g.Expect(k8sClient.Create(ctx, box)).To(Succeed())

	var pvc corev1.PersistentVolumeClaim
	g.Eventually(func() error {
		return k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkspaceName(id)}, &pvc)
	}, timeout, interval).Should(Succeed())
	g.Eventually(func(g Gomega) agentv1.Phase { return boxPhase(g, box) }, timeout, interval).
		Should(Equal(agentv1.PhaseProvisioning))

	// The claim is unbound, so the worker must not exist yet.
	worker := &agentv1.AgentWorker{}
	workerKey := client.ObjectKey{Namespace: ns, Name: agentv1.WorkerName(id)}
	g.Consistently(func() bool {
		return client.IgnoreNotFound(k8sClient.Get(ctx, workerKey, worker)) == nil &&
			k8sClient.Get(ctx, workerKey, worker) != nil
	}, 2*time.Second, interval).Should(BeTrue())

	g.Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &pvc)).To(Succeed())
		pvc.Status.Phase = corev1.ClaimBound
		g.Expect(k8sClient.Status().Update(ctx, &pvc)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	g.Eventually(func() error { return k8sClient.Get(ctx, workerKey, worker) }, timeout, interval).Should(Succeed())
	g.Eventually(func(g Gomega) agentv1.Phase { return boxPhase(g, box) }, timeout, interval).
		Should(Equal(agentv1.PhaseReady))
}

func TestBoxSpecValidationAndPropagation(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	id := string(uuid.NewUUID())
	ns := agentv1.BoxName(id)

	box := newBox(id, waitClass)
	g.Expect(k8sClient.Create(ctx, box)).To(Succeed())
	g.Eventually(func(g Gomega) agentv1.Phase { return boxPhase(g, box) }, timeout, interval).
		Should(Equal(agentv1.PhaseReady))

	other := box.DeepCopy()
	other.Spec.AgentSessionID = string(uuid.NewUUID())
	g.Expect(k8sClient.Update(ctx, other)).To(MatchError(ContainSubstring("immutable")))
	other = box.DeepCopy()
	other.Spec.Worker.Image = "alpine"
	g.Expect(k8sClient.Update(ctx, other)).To(MatchError(ContainSubstring("immutable")))

	// A growing size reaches the workspace; the unbound claim keeps its size.
	box.Spec.Workspace.Size = resource.MustParse("2Gi")
	g.Expect(k8sClient.Update(ctx, box)).To(Succeed())
	var ws agentv1.AgentWorkspace
	g.Eventually(func() string {
		_ = k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkspaceName(id)}, &ws)
		return ws.Spec.Size.String()
	}, timeout, interval).Should(Equal("2Gi"))
	g.Eventually(func(g Gomega) agentv1.Phase { return boxPhase(g, box) }, timeout, interval).
		Should(Equal(agentv1.PhaseReady))
	var pvc corev1.PersistentVolumeClaim
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkspaceName(id)}, &pvc)).To(Succeed())
	g.Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))
}

func TestBoxRequiresWorkspaceAndWorker(t *testing.T) {
	g := NewWithT(t)
	id := string(uuid.NewUUID())

	box := newBox(id, waitClass)
	box.Spec.Worker = agentv1.AgentWorkerTemplate{}
	g.Expect(k8sClient.Create(context.Background(), box)).NotTo(Succeed())

	bad := newBox("not-a-uuid", waitClass)
	g.Expect(k8sClient.Create(context.Background(), bad)).NotTo(Succeed())
}
