package controllers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	benchmarksv1alpha1 "github.com/8terbahn/gpu_benchmark_platform/operator/api/v1alpha1"
	"github.com/8terbahn/gpu_benchmark_platform/operator/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BenchmarkJob Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	Expect(benchmarksv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	// Mock HTTP server simulating the Python FastAPI backend.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/benchmarks":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"job_id": "test-uuid-1234",
				"status": "queued",
			})
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "completed",
				"result": map[string]float64{
					"throughput":            687.5,
					"latency_ms":            4.21,
					"memory_bandwidth_gbps": 1243.7,
				},
			})
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))

	Expect((&controllers.BenchmarkJobReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Log:        ctrl.Log.WithName("BenchmarkJob"),
		HTTPClient: mockServer.Client(),
	}).SetupWithManager(mgr)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	cancel()
	Expect(testEnv.Stop()).To(Succeed())
})

// --- Integration tests ---

var _ = Describe("BenchmarkJob controller", func() {
	const (
		jobName   = "test-h100-benchmark"
		namespace = "default"
		timeout   = 20 * time.Second
		interval  = 250 * time.Millisecond
	)

	It("should drive a BenchmarkJob from Pending to Completed", func() {
		By("creating a BenchmarkJob CR")
		job := &benchmarksv1alpha1.BenchmarkJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: namespace,
			},
			Spec: benchmarksv1alpha1.BenchmarkJobSpec{
				GPUModel:        "H100",
				WorkloadType:    "inference",
				BatchSize:       32,
				SchedulerAPIURL: "http://mock-server",
			},
		}
		Expect(k8sClient.Create(ctx, job)).To(Succeed())

		By("expecting the job to reach Completed phase")
		Eventually(func(g Gomega) {
			var j benchmarksv1alpha1.BenchmarkJob
			g.Expect(k8sClient.Get(ctx,
				client.ObjectKey{Name: jobName, Namespace: namespace}, &j,
			)).To(Succeed())
			g.Expect(j.Status.Phase).To(Equal(controllers.PhaseCompleted))
			g.Expect(j.Status.JobID).To(Equal("test-uuid-1234"))
			g.Expect(j.Status.Throughput).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
	})
})
