package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

const (
	waitClass      = "wait-for-first-consumer"
	immediateClass = "immediate"
)

var k8sClient client.Client

func TestMain(m *testing.M) {
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "k8s", "crds")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := env.Start()
	if err != nil {
		panic(err)
	}
	if err := agentv1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		panic(err)
	}
	for _, r := range []interface{ SetupWithManager(ctrl.Manager) error }{
		&AgentBoxReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		&AgentWorkspaceReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		&AgentWorkerReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
	} {
		if err := r.SetupWithManager(mgr); err != nil {
			panic(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := mgr.Start(ctx); err != nil {
			panic(err)
		}
	}()
	k8sClient = mgr.GetClient()

	direct, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}
	modes := map[string]storagev1.VolumeBindingMode{
		waitClass:      storagev1.VolumeBindingWaitForFirstConsumer,
		immediateClass: storagev1.VolumeBindingImmediate,
	}
	for name, mode := range modes {
		sc := &storagev1.StorageClass{
			ObjectMeta:        metav1.ObjectMeta{Name: name},
			Provisioner:       "kubernetes.io/no-provisioner",
			VolumeBindingMode: &mode,
		}
		if err := direct.Create(ctx, sc); err != nil {
			panic(err)
		}
	}

	code := m.Run()
	cancel()
	_ = env.Stop()
	os.Exit(code)
}
