package factory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
	"github.com/nickdemianchuk/agent-factory/controllers"
)

const testClass = "wait-for-first-consumer"

var testFactory *Factory

func testSpec() agentv1.AgentBoxSpec {
	class := testClass
	return agentv1.AgentBoxSpec{
		Workspace: agentv1.AgentWorkspaceTemplate{Size: resource.MustParse("1Gi"), StorageClassName: &class},
		Worker:    agentv1.AgentWorkerTemplate{Image: "busybox"},
	}
}

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
	direct, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}
	mode := storagev1.VolumeBindingWaitForFirstConsumer
	sc := &storagev1.StorageClass{
		ObjectMeta:        metav1.ObjectMeta{Name: testClass},
		Provisioner:       "kubernetes.io/no-provisioner",
		VolumeBindingMode: &mode,
	}
	if err := direct.Create(ctx, sc); err != nil {
		panic(err)
	}
	testFactory = New(mgr.GetClient())
	testFactory.ReadyTimeout = 30 * time.Second
	testFactory.PollInterval = 100 * time.Millisecond

	code := m.Run()
	cancel()
	_ = env.Stop()
	os.Exit(code)
}

func TestAgentBoxLifecycle(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	s, err := testFactory.Create(ctx, "", testSpec())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(s.Box.Name).To(Equal("agent-box-" + s.ID))
	g.Expect(s.Workspace.Name).To(Equal(agentv1.WorkspaceName))
	g.Expect(s.Worker.Name).To(Equal(agentv1.WorkerName))
	g.Expect(s.Workspace.Namespace).To(Equal("agent-box-" + s.ID))
	g.Expect(s.Box.Status.Phase).To(Equal(agentv1.PhaseReady))
	g.Expect(s.Worker.CreationTimestamp.Time).To(BeTemporally(">=", s.Workspace.CreationTimestamp.Time))

	list, err := testFactory.List(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(list).To(ContainElement(HaveField("ID", s.ID)))

	g.Expect(testFactory.UpdateWorkspace(ctx, s.ID, func(sp *agentv1.AgentWorkspaceTemplate) {
		sp.Size = resource.MustParse("2Gi")
	})).To(Succeed())
	g.Eventually(func() string {
		got, err := testFactory.Get(ctx, s.ID)
		if err != nil || got.Workspace == nil {
			return ""
		}
		return got.Workspace.Spec.Size.String()
	}, 10*time.Second, 100*time.Millisecond).Should(Equal("2Gi"))

	g.Expect(testFactory.Delete(ctx, s.ID)).To(Succeed())
	g.Eventually(func() error {
		_, err := testFactory.Get(ctx, s.ID)
		return err
	}, 20*time.Second, 100*time.Millisecond).Should(MatchError(ErrNotFound))
	g.Expect(testFactory.Delete(ctx, s.ID)).To(MatchError(ErrNotFound))
}

func TestCreateRejectsInvalidAgentBoxID(t *testing.T) {
	g := NewWithT(t)
	_, err := testFactory.Create(context.Background(), "nope", testSpec())
	g.Expect(err).To(HaveOccurred())
}
