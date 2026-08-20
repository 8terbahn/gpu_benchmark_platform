// Package controller implements the GPU Benchmark Crossplane Provider.
//
// # Architecture
//
// This package implements the managed.ExternalConnecter and managed.ExternalClient
// interfaces from crossplane-runtime, which together form the core of a Crossplane
// Provider controller:
//
//	connector.Connect()  →  reads ProviderConfig  →  builds external client
//	external.Observe()   →  GET  /benchmarks/{id}  →  reflects backend state into CR status
//	external.Create()    →  POST /benchmarks        →  submits job, stores jobID in atProvider
//	external.Update()    →  no-op (jobs are immutable once submitted)
//	external.Delete()    →  DELETE /benchmarks/{id} →  cancels job, removes MR
//
// The managed.Reconciler (from crossplane-runtime) calls these methods in a
// level-triggered reconciliation loop, handling retries, status conditions, and
// finalizer lifecycle automatically — this is the key difference from the
// Kubernetes Operator pattern used in the operator/ module, where all that logic
// was written manually. The Provider delegates lifecycle management to
// crossplane-runtime and focuses purely on the external API interaction.
//
// # Observe / Create / Update / Delete contract
//
//   - Observe: must be idempotent. Returns ResourceExists=false if no job ID is
//     stored yet. Returns ResourceUpToDate=false if the spec has changed (not
//     applicable here — benchmark jobs are immutable after creation).
//   - Create: POSTs the job and stores the returned UUID in status.atProvider.jobID.
//     Uses namespace/name as an idempotency key — safe to retry.
//   - Update: intentionally a no-op. A benchmark job cannot be modified after
//     submission; the user must delete and re-create.
//   - Delete: DELETEs the backend job (best-effort) before Crossplane removes the
//     finalizer and garbage-collects the Managed Resource.
package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/8terbahn/gpu_benchmark_platform/provider/api/v1alpha1"
)

// Error strings used for wrapping — keep them as constants so tests can assert on them.
const (
	errNotGPUBenchmarkJob = "managed resource is not a GPUBenchmarkJob"
	errGetProviderConfig  = "cannot get ProviderConfig"
	errObserveBenchmark   = "cannot observe benchmark job in backend API"
	errCreateBenchmark    = "cannot create benchmark job in backend API"
	errDeleteBenchmark    = "cannot delete benchmark job from backend API"
)

// SetupGPUBenchmarkJob registers the GPUBenchmarkJob controller with the
// controller-manager. It wires a managed.Reconciler that delegates external
// resource management to the connector / external client pair defined below.
func SetupGPUBenchmarkJob(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.GPUBenchmarkJobGroupVersionKind.Kind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.GPUBenchmarkJobGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			client:     mgr.GetClient(),
			httpClient: &http.Client{Timeout: 10 * time.Second},
			logger:     l,
		}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(15*time.Second),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.GPUBenchmarkJob{}).
		Complete(r)
}

// ── connector ─────────────────────────────────────────────────────────────────

// connector implements managed.ExternalConnecter.
// Its sole responsibility is to read the ProviderConfig and return an
// initialized ExternalClient bound to that config's scheduler API URL.
type connector struct {
	client     client.Client
	httpClient *http.Client
	logger     logging.Logger
}

// Connect reads the ProviderConfig referenced by the Managed Resource and
// returns an ExternalClient configured to talk to that backend.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.GPUBenchmarkJob)
	if !ok {
		return nil, errors.New(errNotGPUBenchmarkJob)
	}

	// Resolve ProviderConfig reference (defaults to "default" if not specified).
	pcRef := cr.GetProviderConfigReference()
	pcName := "default"
	if pcRef != nil && pcRef.Name != "" {
		pcName = pcRef.Name
	}

	pc := &v1alpha1.ProviderConfig{}
	if err := c.client.Get(ctx, types.NamespacedName{Name: pcName}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	return &external{
		http:         c.httpClient,
		schedulerURL: pc.Spec.SchedulerAPIURL,
		log:          c.logger,
	}, nil
}

// ── external ──────────────────────────────────────────────────────────────────

// external implements managed.ExternalClient.
// Each method corresponds to a CRUD operation on the Python FastAPI backend.
type external struct {
	http         *http.Client
	schedulerURL string
	log          logging.Logger
}

// benchmarkAPIResult mirrors the Python API's GET /benchmarks/{id} response body.
type benchmarkAPIResult struct {
	Status string `json:"status"`
	Result *struct {
		Throughput          float64 `json:"throughput"`
		LatencyMs           float64 `json:"latency_ms"`
		MemoryBandwidthGbps float64 `json:"memory_bandwidth_gbps"`
	} `json:"result"`
}

// Observe fetches the current state of the external benchmark job and maps it
// into the Managed Resource's status.atProvider. It drives the Crossplane
// Ready/Synced conditions based on the backend's reported phase.
//
// Return values:
//   - ResourceExists=false → no job ID known yet; managed reconciler will call Create.
//   - ResourceExists=true, ResourceUpToDate=true → job is in progress or terminal.
//   - ResourceUpToDate=false → never returned here (benchmark specs are immutable).
func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.GPUBenchmarkJob)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotGPUBenchmarkJob)
	}

	// If we have no jobID stored, the external resource has not been created yet.
	if cr.Status.AtProvider.JobID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	url := fmt.Sprintf("%s/benchmarks/%s", e.schedulerURL, cr.Status.AtProvider.JobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveBenchmark)
	}

	resp, err := e.http.Do(req)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveBenchmark)
	}
	defer resp.Body.Close()

	// 404 means the backend no longer knows about this job — treat as deleted.
	if resp.StatusCode == http.StatusNotFound {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	var result benchmarkAPIResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveBenchmark)
	}

	// Populate atProvider from the backend response.
	cr.Status.AtProvider.Phase = result.Status

	switch result.Status {
	case "completed":
		if result.Result != nil {
			cr.Status.AtProvider.Throughput = &result.Result.Throughput
			cr.Status.AtProvider.LatencyMs = &result.Result.LatencyMs
			cr.Status.AtProvider.MemoryBandwidthGbps = &result.Result.MemoryBandwidthGbps
		}
		// Mark the Managed Resource as Available (standard Crossplane condition).
		cr.SetConditions(xpv1.Available())
		e.log.Debug("benchmark completed",
			"jobID", cr.Status.AtProvider.JobID,
			"throughput", cr.Status.AtProvider.Throughput)

	case "failed":
		cr.SetConditions(xpv1.Unavailable())

	default:
		// queued or running — job exists but is not yet ready.
		cr.SetConditions(xpv1.Creating())
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

// Create submits a new benchmark job to the Python API backend.
// It stores the returned job UUID in status.atProvider.jobID so subsequent
// Observe calls can poll for completion.
//
// The namespace/name pair is used as an idempotency key — if the provider
// retries Create after a transient failure, the backend will return the
// existing job rather than creating a duplicate.
func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.GPUBenchmarkJob)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotGPUBenchmarkJob)
	}

	payload := map[string]interface{}{
		"gpu_model":       cr.Spec.ForProvider.GPUModel,
		"workload_type":   cr.Spec.ForProvider.WorkloadType,
		"batch_size":      cr.Spec.ForProvider.BatchSize,
		"idempotency_key": fmt.Sprintf("%s/%s", cr.GetNamespace(), cr.GetName()),
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.schedulerURL+"/benchmarks", bytes.NewReader(body))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateBenchmark)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.http.Do(req)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateBenchmark)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return managed.ExternalCreation{}, errors.Errorf("%s: unexpected status %d from backend",
			errCreateBenchmark, resp.StatusCode)
	}

	var result struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateBenchmark)
	}

	cr.Status.AtProvider.JobID = result.JobID
	cr.Status.AtProvider.Phase = "queued"
	cr.SetConditions(xpv1.Creating())

	e.log.Debug("benchmark job submitted", "jobID", result.JobID,
		"gpu", cr.Spec.ForProvider.GPUModel)

	return managed.ExternalCreation{}, nil
}

// Update is intentionally a no-op.
//
// Benchmark jobs are immutable once submitted to the backend — the execution
// parameters (GPU model, workload type, batch size) cannot be changed after
// the job has been queued. Users must delete and re-create the Managed Resource
// to run a benchmark with different parameters.
//
// This is a deliberate design constraint, not a limitation: it preserves the
// integrity of benchmark results by preventing mid-run parameter changes.
func (e *external) Update(_ context.Context, _ resource.Managed) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, nil
}

// Delete cancels the benchmark job in the Python API backend (best-effort).
// If the backend returns an error (e.g., job already completed or not found),
// the error is logged but not surfaced — Crossplane will proceed to remove the
// finalizer and delete the Managed Resource regardless.
func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.GPUBenchmarkJob)
	if !ok {
		return errors.New(errNotGPUBenchmarkJob)
	}

	// Nothing to cancel if the job was never submitted.
	if cr.Status.AtProvider.JobID == "" {
		return nil
	}

	url := fmt.Sprintf("%s/benchmarks/%s", e.schedulerURL, cr.Status.AtProvider.JobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		// Log and allow deletion to proceed — the backend job may already be gone.
		e.log.Debug("failed to build cancel request; proceeding with MR deletion",
			"jobID", cr.Status.AtProvider.JobID, "error", err)
		return nil
	}

	resp, err := e.http.Do(req)
	if err != nil {
		e.log.Debug("failed to cancel backend job (best-effort); proceeding with MR deletion",
			"jobID", cr.Status.AtProvider.JobID, "error", err)
		return nil
	}
	resp.Body.Close()

	e.log.Debug("benchmark job cancelled", "jobID", cr.Status.AtProvider.JobID,
		"httpStatus", resp.StatusCode)
	return nil
}
