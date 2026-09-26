package controllers

import (
	"context"
	"maps"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

const (
	// BoxServiceAccount is the service account agents in a box run as.
	BoxServiceAccount = "agent"
	// BoxRole is the role granted to the box service account.
	BoxRole = "agent"
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

// Reconcile drives an AgentBox toward its desired state.
func (r *AgentBoxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var box agentv1.AgentBox
	if err := r.Get(ctx, req.NamespacedName, &box); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !box.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, &box)
	}
	// Patch instead of Update: a cached read can lag behind our own writes, and a stale
	// resourceVersion would fail with a conflict.
	base := box.DeepCopy()
	if controllerutil.AddFinalizer(&box, agentv1.Finalizer) {
		if err := r.Patch(ctx, &box, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
	}
	base = box.DeepCopy()

	if err := r.reconcileResources(ctx, &box); err != nil {
		log.Error(err, "Failed to provision box resources")
		box.Status.Phase = agentv1.PhaseFailed
		setReady(&box.Status.Conditions, box.Generation, false, "ProvisionFailed", err.Error())
		box.Status.ObservedGeneration = box.Generation
		if uerr := r.Status().Patch(ctx, &box, client.MergeFrom(base)); uerr != nil {
			log.Error(uerr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	box.Status.Phase = agentv1.PhaseReady
	box.Status.Namespace = agentv1.BoxName(box.Spec.AgentSessionID)
	box.Status.ServiceAccountName = BoxServiceAccount
	box.Status.ObservedGeneration = box.Generation
	setReady(&box.Status.Conditions, box.Generation, true, "Provisioned", "namespace and RBAC are in place")
	return ctrl.Result{}, r.Status().Patch(ctx, &box, client.MergeFrom(base))
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

// finalize deletes the box namespace, which removes every resource inside it.
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

// SetupWithManager registers the reconciler with the manager.
func (r *AgentBoxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentv1.AgentBox{}).
		Owns(&corev1.Namespace{}).
		Named("agentbox").
		Complete(r)
}
