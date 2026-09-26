package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

// AgentWorkspaceReconciler provisions the PVC of an AgentWorkspace.
type AgentWorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkspaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile drives an AgentWorkspace to its desired state.
func (r *AgentWorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var ws agentv1.AgentWorkspace
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !ws.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	base := ws.DeepCopy()

	id := ws.Spec.AgentSessionID
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.WorkspaceName(id), Namespace: ws.Namespace},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		pvc.Labels = mergeLabels(pvc.Labels, sessionLabels(id))
		if pvc.CreationTimestamp.IsZero() {
			modes := ws.Spec.AccessModes
			if len(modes) == 0 {
				modes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			}
			pvc.Spec.AccessModes = modes
			pvc.Spec.StorageClassName = ws.Spec.StorageClassName
			pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: ws.Spec.Size}
		} else if cur := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ws.Spec.Size.Cmp(cur) > 0 {
			// Only the storage request is mutable, and only once the volume can expand.
			expand, err := r.canExpand(ctx, pvc)
			if err != nil {
				return err
			}
			if expand {
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = ws.Spec.Size
			}
		}
		return controllerutil.SetControllerReference(&ws, pvc, r.Scheme)
	}); err != nil {
		log.Error(err, "Failed to reconcile claim")
		ws.Status.Phase = agentv1.PhaseFailed
		setReady(&ws.Status.Conditions, ws.Generation, false, "ClaimFailed", err.Error())
		ws.Status.ObservedGeneration = ws.Generation
		if uerr := r.Status().Patch(ctx, &ws, client.MergeFrom(base)); uerr != nil {
			log.Error(uerr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	ready, reason, message, err := r.claimState(ctx, pvc)
	if err != nil {
		return ctrl.Result{}, err
	}
	ws.Status.ClaimName = pvc.Name
	ws.Status.ObservedGeneration = ws.Generation
	ws.Status.Phase = agentv1.PhaseProvisioning
	switch {
	case ready:
		ws.Status.Phase = agentv1.PhaseReady
	case reason == "ClaimLost":
		ws.Status.Phase = agentv1.PhaseFailed
	}
	setReady(&ws.Status.Conditions, ws.Generation, ready, reason, message)
	return ctrl.Result{}, r.Status().Patch(ctx, &ws, client.MergeFrom(base))
}

// claimState reports whether the claim is usable. A Pending claim on a
// WaitForFirstConsumer class counts as ready: it only binds once a pod mounts it.
func (r *AgentWorkspaceReconciler) claimState(
	ctx context.Context, pvc *corev1.PersistentVolumeClaim,
) (ready bool, reason, message string, err error) {
	switch pvc.Status.Phase {
	case corev1.ClaimBound:
		return true, "ClaimBound", "claim is bound", nil
	case corev1.ClaimLost:
		return false, "ClaimLost", "backing volume was lost", nil
	}
	sc, err := r.storageClass(ctx, pvc)
	if err != nil {
		return false, "", "", err
	}
	if sc == nil {
		return false, "WaitingForClaim", "claim is pending and has no storage class", nil
	}
	if sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
		return true, "ClaimWaitingForConsumer", "claim binds when the worker is scheduled", nil
	}
	return false, "WaitingForClaim", fmt.Sprintf("claim is pending on storage class %s", sc.Name), nil
}

func (r *AgentWorkspaceReconciler) canExpand(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	if pvc.Status.Phase != corev1.ClaimBound {
		return false, nil
	}
	sc, err := r.storageClass(ctx, pvc)
	if err != nil || sc == nil {
		return false, err
	}
	return sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion, nil
}

// storageClass returns the claim's storage class, or nil if it has none or it does not exist.
func (r *AgentWorkspaceReconciler) storageClass(
	ctx context.Context, pvc *corev1.PersistentVolumeClaim,
) (*storagev1.StorageClass, error) {
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
		return nil, nil
	}
	var sc storagev1.StorageClass
	if err := r.Get(ctx, client.ObjectKey{Name: *pvc.Spec.StorageClassName}, &sc); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return &sc, nil
}

// SetupWithManager registers the reconciler with mgr.
func (r *AgentWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentv1.AgentWorkspace{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Named("agentworkspace").
		Complete(r)
}
