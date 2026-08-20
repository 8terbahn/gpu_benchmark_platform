package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	benchmarksv1alpha1 "github.com/8terbahn/gpu_benchmark_platform/operator/api/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	benchmarkJobFinalizer = "benchmarks.gpu-platform.io/finalizer"

	PhasePending   = "Pending"
	PhaseSubmitted = "Submitted"
	PhaseRunning   = "Running"
	PhaseCompleted = "Completed"
	PhaseFailed    = "Failed"

	pollInterval   = 15 * time.Second
	retryInterval  = 30 * time.Second
)

// BenchmarkJobReconciler reconciles BenchmarkJob objects.
//
// This controller implements the Kubernetes Operator pattern (controller-runtime).
// It continuously watches BenchmarkJob CRs, submits them to the Python FastAPI backend,
// polls for results, and drives the workload through a phase state machine.
//
// The Python backend is treated as a black-box execution engine — all scheduling
// state is owned by the operator and stored in the CR's status subresource.
//
// +kubebuilder:rbac:groups=benchmarks.gpu-platform.io,resources=benchmarkjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=benchmarks.gpu-platform.io,resources=benchmarkjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=benchmarks.gpu-platform.io,resources=benchmarkjobs/finalizers,verbs=update
type BenchmarkJobReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Log        logr.Logger
	HTTPClient *http.Client
}

// SetupWithManager registers this controller with the controller-manager.
func (r *BenchmarkJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.HTTPClient == nil {
		r.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&benchmarksv1alpha1.BenchmarkJob{}).
		Complete(r)
}

// Reconcile is the core reconciliation loop. It is invoked whenever a BenchmarkJob
// is created, updated, or deleted. Each call is idempotent.
//
// State machine:
//
//	(empty) -> Pending -> Submitted -> Running -> Completed
//	                                         \--> Failed
func (r *BenchmarkJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("benchmarkjob", req.NamespacedName)

	// 1. Fetch the BenchmarkJob resource.
	var job benchmarksv1alpha1.BenchmarkJob
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get BenchmarkJob: %w", err)
	}

	// 2. Handle deletion: cancel the backend job and remove the finalizer.
	if !job.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, log, &job)
	}

	// 3. Ensure the finalizer is registered.
	if !controllerutil.ContainsFinalizer(&job, benchmarkJobFinalizer) {
		controllerutil.AddFinalizer(&job, benchmarkJobFinalizer)
		if err := r.Update(ctx, &job); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Drive the phase state machine.
	switch job.Status.Phase {
	case "", PhasePending:
		return r.handlePending(ctx, log, &job)
	case PhaseSubmitted:
		return r.handleSubmitted(ctx, log, &job)
	case PhaseRunning:
		return r.handleRunning(ctx, log, &job)
	case PhaseCompleted, PhaseFailed:
		// Terminal states — nothing more to do.
		return ctrl.Result{}, nil
	default:
		log.Info("unknown phase; resetting to Pending", "phase", job.Status.Phase)
		return r.setPhase(ctx, &job, PhasePending, "unknown phase reset by operator")
	}
}

// handlePending submits the benchmark job to the Python FastAPI backend via POST /benchmarks.
// On success it records the backend job UUID and transitions to Submitted.
// On transient errors it re-queues with backoff.
func (r *BenchmarkJobReconciler) handlePending(
	ctx context.Context,
	log logr.Logger,
	job *benchmarksv1alpha1.BenchmarkJob,
) (ctrl.Result, error) {
	apiURL := job.Spec.SchedulerAPIURL
	if apiURL == "" {
		apiURL = "http://gpu-benchmark-api:8000"
	}

	payload := map[string]interface{}{
		"gpu_model":     job.Spec.GPUModel,
		"workload_type": job.Spec.WorkloadType,
		"batch_size":    job.Spec.BatchSize,
		// Idempotency key derived from the CR identity — safe to retry.
		"idempotency_key": fmt.Sprintf("%s/%s", job.Namespace, job.Name),
	}

	body, _ := json.Marshal(payload)
	resp, err := r.HTTPClient.Post(apiURL+"/benchmarks", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Error(err, "failed to submit job to Python API; will retry")
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		log.Info("unexpected status from Python API; will retry",
			"status", resp.StatusCode, "body", string(raw))
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	var result struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	now := metav1.Now()
	job.Status.JobID = result.JobID
	job.Status.StartTime = &now
	log.Info("benchmark job submitted to Python API", "jobID", result.JobID)
	return r.setPhase(ctx, job, PhaseSubmitted,
		fmt.Sprintf("submitted to backend; jobID=%s", result.JobID))
}

// handleSubmitted polls the backend until the job transitions to running.
func (r *BenchmarkJobReconciler) handleSubmitted(
	ctx context.Context,
	log logr.Logger,
	job *benchmarksv1alpha1.BenchmarkJob,
) (ctrl.Result, error) {
	status, err := r.pollBackend(job)
	if err != nil {
		log.Error(err, "poll error; will retry")
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	switch status {
	case "running":
		log.Info("backend job is running", "jobID", job.Status.JobID)
		return r.setPhase(ctx, job, PhaseRunning, "execution in progress")
	case "queued":
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	case "failed":
		return r.handleFailure(ctx, log, job)
	default:
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}
}

// handleRunning polls the backend until the job completes or fails.
func (r *BenchmarkJobReconciler) handleRunning(
	ctx context.Context,
	log logr.Logger,
	job *benchmarksv1alpha1.BenchmarkJob,
) (ctrl.Result, error) {
	apiURL := job.Spec.SchedulerAPIURL
	if apiURL == "" {
		apiURL = "http://gpu-benchmark-api:8000"
	}

	resp, err := r.HTTPClient.Get(fmt.Sprintf("%s/benchmarks/%s", apiURL, job.Status.JobID))
	if err != nil {
		log.Error(err, "poll error; will retry")
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}
	defer resp.Body.Close()

	var result struct {
		Status string   `json:"status"`
		Result *struct {
			Throughput          float64 `json:"throughput"`
			LatencyMs           float64 `json:"latency_ms"`
			MemoryBandwidthGbps float64 `json:"memory_bandwidth_gbps"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	switch result.Status {
	case "completed":
		now := metav1.Now()
		job.Status.CompletionTime = &now
		if result.Result != nil {
			job.Status.Throughput = &result.Result.Throughput
			job.Status.LatencyMs = &result.Result.LatencyMs
			job.Status.MemoryBandwidthGbps = &result.Result.MemoryBandwidthGbps
		}
		log.Info("benchmark completed",
			"jobID", job.Status.JobID,
			"throughput", job.Status.Throughput)
		return r.setPhase(ctx, job, PhaseCompleted, "benchmark execution successful")
	case "failed":
		return r.handleFailure(ctx, log, job)
	default:
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}
}

// handleFailure increments the retry counter and either re-submits or transitions to Failed.
func (r *BenchmarkJobReconciler) handleFailure(
	ctx context.Context,
	log logr.Logger,
	job *benchmarksv1alpha1.BenchmarkJob,
) (ctrl.Result, error) {
	maxRetries := job.Spec.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	if job.Status.RetryCount < maxRetries {
		job.Status.RetryCount++
		job.Status.JobID = "" // force re-submission with new idempotency variant
		log.Info("retrying failed benchmark",
			"attempt", job.Status.RetryCount, "maxRetries", maxRetries)
		return r.setPhase(ctx, job, PhasePending,
			fmt.Sprintf("retry %d/%d after backend failure", job.Status.RetryCount, maxRetries))
	}

	now := metav1.Now()
	job.Status.CompletionTime = &now
	log.Info("benchmark permanently failed after max retries", "retries", job.Status.RetryCount)
	return r.setPhase(ctx, job, PhaseFailed,
		fmt.Sprintf("failed after %d retries", job.Status.RetryCount))
}

// handleDeletion cancels the backend job (best-effort) and removes the finalizer.
func (r *BenchmarkJobReconciler) handleDeletion(
	ctx context.Context,
	log logr.Logger,
	job *benchmarksv1alpha1.BenchmarkJob,
) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(job, benchmarkJobFinalizer) {
		// Best-effort cancellation: notify backend if job was submitted.
		if job.Status.JobID != "" &&
			job.Status.Phase != PhaseCompleted &&
			job.Status.Phase != PhaseFailed {
			apiURL := job.Spec.SchedulerAPIURL
			if apiURL == "" {
				apiURL = "http://gpu-benchmark-api:8000"
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodDelete,
				fmt.Sprintf("%s/benchmarks/%s", apiURL, job.Status.JobID), nil)
			if resp, err := r.HTTPClient.Do(req); err == nil {
				resp.Body.Close()
			} else {
				log.Error(err, "failed to cancel backend job on delete (best-effort)")
			}
		}
		controllerutil.RemoveFinalizer(job, benchmarkJobFinalizer)
		if err := r.Update(ctx, job); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

// pollBackend fetches the current status string from the Python API.
func (r *BenchmarkJobReconciler) pollBackend(job *benchmarksv1alpha1.BenchmarkJob) (string, error) {
	apiURL := job.Spec.SchedulerAPIURL
	if apiURL == "" {
		apiURL = "http://gpu-benchmark-api:8000"
	}
	resp, err := r.HTTPClient.Get(fmt.Sprintf("%s/benchmarks/%s", apiURL, job.Status.JobID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Status, nil
}

// setPhase is a convenience helper that updates status.Phase and status.Message.
func (r *BenchmarkJobReconciler) setPhase(
	ctx context.Context,
	job *benchmarksv1alpha1.BenchmarkJob,
	phase, message string,
) (ctrl.Result, error) {
	job.Status.Phase = phase
	job.Status.Message = message
	if err := r.Status().Update(ctx, job); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status (phase=%s): %w", phase, err)
	}
	return ctrl.Result{}, nil
}
