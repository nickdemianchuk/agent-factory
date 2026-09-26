package controllers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
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
	workerContainer  = "worker"
	workspaceVolume  = "workspace"
	defaultMountPath = "/workspace"
)

// AgentWorkerReconciler provisions the Pod of an AgentWorker.
type AgentWorkerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentfactory.io,resources=agentworkers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile drives an AgentWorker to its desired state.
func (r *AgentWorkerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var worker agentv1.AgentWorker
	if err := r.Get(ctx, req.NamespacedName, &worker); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !worker.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	base := worker.DeepCopy()

	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: agentv1.WorkerName, Namespace: worker.Namespace}}
	if err := r.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		pod = r.buildPod(&worker, BoxServiceAccount)
		if err := controllerutil.SetControllerReference(&worker, pod, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := ignoreAlreadyExists(r.Create(ctx, pod)); err != nil {
			log.Error(err, "Failed to create worker pod")
			worker.Status.Phase = agentv1.PhaseFailed
			setReady(&worker.Status.Conditions, worker.Generation, false, "PodFailed", err.Error())
			worker.Status.ObservedGeneration = worker.Generation
			if uerr := r.Status().Patch(ctx, &worker, client.MergeFrom(base)); uerr != nil {
				log.Error(uerr, "Failed to update status")
			}
			return ctrl.Result{}, err
		}
	}

	worker.Status.PodName = pod.Name
	worker.Status.ObservedGeneration = worker.Generation
	switch pod.Status.Phase {
	case corev1.PodRunning:
		worker.Status.Phase = agentv1.PhaseReady
		setReady(&worker.Status.Conditions, worker.Generation, true, "PodRunning", "worker pod is running")
	case corev1.PodSucceeded:
		worker.Status.Phase = agentv1.PhaseCompleted
		setReady(&worker.Status.Conditions, worker.Generation, false, "PodCompleted", "worker pod completed")
	case corev1.PodFailed:
		worker.Status.Phase = agentv1.PhaseFailed
		setReady(&worker.Status.Conditions, worker.Generation, false, "PodFailed", "worker pod failed")
	default:
		worker.Status.Phase = agentv1.PhaseProvisioning
		setReady(&worker.Status.Conditions, worker.Generation, false, "PodStarting",
			"worker pod is "+string(pod.Status.Phase))
	}
	return ctrl.Result{}, r.Status().Patch(ctx, &worker, client.MergeFrom(base))
}

func (r *AgentWorkerReconciler) buildPod(w *agentv1.AgentWorker, serviceAccount string) *corev1.Pod {
	id := w.Spec.AgentBoxID
	mountPath := w.Spec.WorkspaceMountPath
	if mountPath == "" {
		mountPath = defaultMountPath
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentv1.WorkerName,
			Namespace: w.Namespace,
			Labels:    resourceLabels(id, componentWorker),
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: serviceAccount,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:         workerContainer,
				Image:        w.Spec.Image,
				Command:      w.Spec.Command,
				Args:         w.Spec.Args,
				Env:          w.Spec.Env,
				Resources:    w.Spec.Resources,
				VolumeMounts: []corev1.VolumeMount{{Name: workspaceVolume, MountPath: mountPath}},
			}},
			Volumes: []corev1.Volume{{
				Name: workspaceVolume,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: agentv1.WorkspaceName,
					},
				},
			}},
		},
	}
}

// SetupWithManager registers the reconciler with mgr.
func (r *AgentWorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentv1.AgentWorker{}).
		Owns(&corev1.Pod{}).
		Named("agentworker").
		Complete(r)
}
