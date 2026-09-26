package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

const requeueWaiting = 2 * time.Second

type AgentWorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkspaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

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
	box, err := getBox(ctx, r.Client, id)
	if err != nil {
		return ctrl.Result{}, err
	}
	if box == nil || !isReady(box.Status.Conditions, box.Generation) || ws.Namespace != box.Status.Namespace {
		ws.Status.Phase = agentv1.PhasePending
		setReady(&ws.Status.Conditions, ws.Generation, false, "WaitingForBox", "box is not ready")
		ws.Status.ObservedGeneration = ws.Generation
		if err := r.Status().Patch(ctx, &ws, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueWaiting}, nil
	}

	if err := controllerutil.SetOwnerReference(box, &ws, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Patch(ctx, &ws, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, err
	}
	base = ws.DeepCopy()

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
		}
		// Only the storage request is mutable.
		if cur, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; !ok || ws.Spec.Size.Cmp(cur) > 0 {
			pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: ws.Spec.Size}
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

	// WaitForFirstConsumer claims stay Pending until mounted; that still counts as ready.
	ws.Status.ClaimName = pvc.Name
	ws.Status.ObservedGeneration = ws.Generation
	if pvc.Status.Phase == corev1.ClaimLost {
		ws.Status.Phase = agentv1.PhaseFailed
		setReady(&ws.Status.Conditions, ws.Generation, false, "ClaimLost", "backing volume was lost")
	} else {
		ws.Status.Phase = agentv1.PhaseReady
		setReady(&ws.Status.Conditions, ws.Generation, true, "ClaimCreated", "claim is "+string(pvc.Status.Phase))
	}
	return ctrl.Result{}, r.Status().Patch(ctx, &ws, client.MergeFrom(base))
}

func (r *AgentWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentv1.AgentWorkspace{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Named("agentworkspace").
		Complete(r)
}
