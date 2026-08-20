// Command provider is the entry point for the GPU Benchmark Crossplane Provider.
//
// It starts a controller-manager that runs the GPUBenchmarkJob managed reconciler.
// The manager watches GPUBenchmarkJob Managed Resources cluster-wide and delegates
// external lifecycle management (Observe / Create / Update / Delete) to the Python
// FastAPI benchmark backend, as configured in the referenced ProviderConfig.
package main

import (
	"os"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1alpha1 "github.com/8terbahn/gpu_benchmark_platform/provider/api/v1alpha1"
	"github.com/8terbahn/gpu_benchmark_platform/provider/internal/controller"
)

func main() {
	// ── Logging ───────────────────────────────────────────────────────────────
	zl := zap.New(zap.UseDevMode(false))
	ctrl.SetLogger(zl)
	log := logging.NewLogrLogger(zl.WithName("provider-gpu-benchmark"))

	// ── Scheme ────────────────────────────────────────────────────────────────
	// Register both the standard Kubernetes types and the provider's custom types.
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(v1alpha1.AddToScheme(s))

	// ── Manager ───────────────────────────────────────────────────────────────
	// The manager multiplexes leader election, metrics, and health endpoints
	// for all controllers registered below.
	syncPeriod := 1 * time.Hour
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 s,
		Metrics:                metricsserver.Options{BindAddress: ":8080"},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         true,
		LeaderElectionID:       "provider-gpu-benchmark.gpu.platform.io",
		LeaderElectionNamespace: "gpu-benchmark",
		Cache:                  cache.Options{SyncPeriod: &syncPeriod},
	})
	if err != nil {
		log.Info("cannot create manager", "error", err)
		os.Exit(1)
	}

	// ── Health checks ─────────────────────────────────────────────────────────
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Info("cannot add healthz check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Info("cannot add readyz check", "error", err)
		os.Exit(1)
	}

	// ── Controllers ───────────────────────────────────────────────────────────
	if err := controller.SetupGPUBenchmarkJob(mgr, log); err != nil {
		log.Info("cannot setup GPUBenchmarkJob controller", "error", err)
		os.Exit(1)
	}

	// ── Start ─────────────────────────────────────────────────────────────────
	log.Info("Starting GPU Benchmark Provider")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Info("provider exited with error", "error", err)
		os.Exit(1)
	}
}
