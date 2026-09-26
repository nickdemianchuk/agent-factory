package factory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
	"github.com/nickdemianchuk/agent-factory/controllers"
)

var testFactory *Factory

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
		&controllers.AgentBoxReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		&controllers.AgentWorkspaceReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		&controllers.AgentWorkerReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
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
	testFactory = New(mgr.GetClient())
	testFactory.ReadyTimeout = 30 * time.Second
	testFactory.PollInterval = 100 * time.Millisecond

	code := m.Run()
	cancel()
	_ = env.Stop()
	os.Exit(code)
}

func TestSessionLifecycle(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	s, err := testFactory.Create(ctx, "", Spec{
		Workspace: agentv1.AgentWorkspaceSpec{Size: resource.MustParse("1Gi")},
		Worker:    agentv1.AgentWorkerSpec{Image: "busybox"},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(s.Box.Name).To(Equal("box-" + s.ID))
	g.Expect(s.Workspace.Name).To(Equal("workspace-" + s.ID))
	g.Expect(s.Worker.Name).To(Equal("worker-" + s.ID))
	g.Expect(s.Workspace.Namespace).To(Equal("box-" + s.ID))
	// Ordering: the workspace was ready before the worker was created.
	g.Expect(s.Workspace.Status.Phase).To(Equal(agentv1.PhaseReady))
	g.Expect(s.Worker.CreationTimestamp.Time).To(BeTemporally(">=", s.Workspace.CreationTimestamp.Time))

	// The client reads from a cache, so allow it to observe the new worker.
	var got *Session
	g.Eventually(func() *agentv1.AgentWorker {
		got, err = testFactory.Get(ctx, s.ID)
		if err != nil {
			return nil
		}
		return got.Worker
	}, 10*time.Second, 100*time.Millisecond).ShouldNot(BeNil())
	g.Expect(got.Workspace).NotTo(BeNil())

	list, err := testFactory.List(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(list).To(ContainElement(HaveField("ID", s.ID)))

	g.Expect(testFactory.UpdateWorkspace(ctx, s.ID, func(sp *agentv1.AgentWorkspaceSpec) {
		sp.Size = resource.MustParse("2Gi")
	})).To(Succeed())
	g.Eventually(func() string {
		got, _ = testFactory.Get(ctx, s.ID)
		return got.Workspace.Spec.Size.String()
	}, 10*time.Second, 100*time.Millisecond).Should(Equal("2Gi"))

	g.Expect(testFactory.UpdateWorker(ctx, s.ID, func(sp *agentv1.AgentWorkerSpec) {
		sp.Args = []string{"--verbose"}
	})).To(Succeed())

	g.Expect(testFactory.Delete(ctx, s.ID)).To(Succeed())
	g.Eventually(func() error {
		_, err := testFactory.Get(ctx, s.ID)
		return err
	}, 20*time.Second, 100*time.Millisecond).Should(MatchError(ErrNotFound))
	g.Expect(testFactory.Delete(ctx, s.ID)).To(MatchError(ErrNotFound))
}

func TestCreateRejectsInvalidSessionID(t *testing.T) {
	g := NewWithT(t)
	_, err := testFactory.Create(context.Background(), "nope", Spec{
		Workspace: agentv1.AgentWorkspaceSpec{Size: resource.MustParse("1Gi")},
		Worker:    agentv1.AgentWorkerSpec{Image: "busybox"},
	})
	g.Expect(err).To(HaveOccurred())
}
