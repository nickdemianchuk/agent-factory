package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
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

	code := m.Run()
	cancel()
	_ = env.Stop()
	os.Exit(code)
}
