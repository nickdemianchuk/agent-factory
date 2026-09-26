// Package controllers reconciles the agentfactory.io resources.
package controllers

import (
	"context"
	"maps"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

// BoxServiceAccount and BoxRole name the service account and role of every box.
const (
	BoxServiceAccount = "agent"
	BoxRole           = "agent"
)

// AgentBoxReconciler provisions the namespace and RBAC of an AgentBox.
type AgentBoxReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agentfactory.io,resources=agentboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentfactory.io,resources=agentboxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentfactory.io,resources=agentboxes/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=escalate;bind

// Reconcile drives an AgentBox to its desired state.
func (r *AgentBoxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var box agentv1.AgentBox
	if err := r.Get(ctx, req.NamespacedName, &box); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !box.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, &box)
	}
	// Patch, not Update: cached reads can be stale.
	base := box.DeepCopy()
	if controllerutil.AddFinalizer(&box, agentv1.Finalizer) {
		if err := r.Patch(ctx, &box, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
	}
	base = box.DeepCopy()

	ready, reason, message, err := r.provision(ctx, &box)
	if err != nil {
		log.Error(err, "Failed to provision box")
		box.Status.Phase = agentv1.PhaseFailed
		setReady(&box.Status.Conditions, box.Generation, false, "ProvisionFailed", err.Error())
		box.Status.ObservedGeneration = box.Generation
		if uerr := r.Status().Patch(ctx, &box, client.MergeFrom(base)); uerr != nil {
			log.Error(uerr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	box.Status.Namespace = agentv1.BoxName(box.Spec.AgentSessionID)
	box.Status.ServiceAccountName = BoxServiceAccount
	box.Status.ObservedGeneration = box.Generation
	box.Status.Phase = agentv1.PhaseProvisioning
	if ready {
		box.Status.Phase = agentv1.PhaseReady
	}
	setReady(&box.Status.Conditions, box.Generation, ready, reason, message)
	return ctrl.Result{}, r.Status().Patch(ctx, &box, client.MergeFrom(base))
}

// provision creates the namespace, RBAC, workspace and worker in that order,
// holding back the worker until the workspace is ready.
func (r *AgentBoxReconciler) provision(
	ctx context.Context, box *agentv1.AgentBox,
) (ready bool, reason, message string, err error) {
	if err := r.reconcileResources(ctx, box); err != nil {
		return false, "", "", err
	}

	id := box.Spec.AgentSessionID
	ns := agentv1.BoxName(id)
	ws := &agentv1.AgentWorkspace{ObjectMeta: metav1.ObjectMeta{Name: agentv1.WorkspaceName(id), Namespace: ns}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ws, func() error {
		ws.Labels = mergeLabels(ws.Labels, sessionLabels(id))
		ws.Spec = agentv1.AgentWorkspaceSpec{
			AgentSessionID:         id,
			AgentWorkspaceTemplate: *box.Spec.Workspace.DeepCopy(),
		}
		return controllerutil.SetControllerReference(box, ws, r.Scheme)
	}); err != nil {
		return false, "", "", err
	}
	if !isReady(ws.Status.Conditions, ws.Generation) {
		return false, "WaitingForWorkspace", workspaceMessage(ws), nil
	}

	worker := &agentv1.AgentWorker{ObjectMeta: metav1.ObjectMeta{Name: agentv1.WorkerName(id), Namespace: ns}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, worker, func() error {
		worker.Labels = mergeLabels(worker.Labels, sessionLabels(id))
		worker.Spec = agentv1.AgentWorkerSpec{
			AgentSessionID:      id,
			AgentWorkerTemplate: *box.Spec.Worker.DeepCopy(),
		}
		return controllerutil.SetControllerReference(box, worker, r.Scheme)
	}); err != nil {
		return false, "", "", err
	}
	return true, "Provisioned", "namespace, workspace and worker are in place", nil
}

func workspaceMessage(ws *agentv1.AgentWorkspace) string {
	if c := meta.FindStatusCondition(ws.Status.Conditions, agentv1.ConditionReady); c != nil && c.Message != "" {
		return "workspace: " + c.Message
	}
	return "workspace is not ready"
}

func (r *AgentBoxReconciler) reconcileResources(ctx context.Context, box *agentv1.AgentBox) error {
	name := agentv1.BoxName(box.Spec.AgentSessionID)
	labels := sessionLabels(box.Spec.AgentSessionID)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		ns.Labels = mergeLabels(ns.Labels, labels)
		return controllerutil.SetControllerReference(box, ns, r.Scheme)
	}); err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: BoxServiceAccount, Namespace: name}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		sa.Labels = mergeLabels(sa.Labels, labels)
		return nil
	}); err != nil {
		return err
	}

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: BoxRole, Namespace: name}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		role.Labels = mergeLabels(role.Labels, labels)
		role.Rules = box.Spec.Rules
		return nil
	}); err != nil {
		return err
	}

	binding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: BoxRole, Namespace: name}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, binding, func() error {
		binding.Labels = mergeLabels(binding.Labels, labels)
		binding.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: BoxRole}
		binding.Subjects = []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: BoxServiceAccount, Namespace: name}}
		return nil
	})
	return err
}

func (r *AgentBoxReconciler) finalize(ctx context.Context, box *agentv1.AgentBox) error {
	if !controllerutil.ContainsFinalizer(box, agentv1.Finalizer) {
		return nil
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(box.Spec.AgentSessionID)}}
	if err := r.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	base := box.DeepCopy()
	controllerutil.RemoveFinalizer(box, agentv1.Finalizer)
	return r.Patch(ctx, box, client.MergeFrom(base))
}

func mergeLabels(dst, src map[string]string) map[string]string {
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	maps.Copy(dst, src)
	return dst
}

// SetupWithManager registers the reconciler with mgr.
func (r *AgentBoxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentv1.AgentBox{}).
		Owns(&corev1.Namespace{}).
		Owns(&agentv1.AgentWorkspace{}).
		Owns(&agentv1.AgentWorker{}).
		Named("agentbox").
		Complete(r)
}
