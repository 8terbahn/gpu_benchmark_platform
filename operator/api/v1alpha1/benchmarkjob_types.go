package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BenchmarkJob is the Schema for declarative GPU benchmark execution.
//
// Teams submit a BenchmarkJob resource to declare a GPU benchmark request.
// The operator continuously reconciles the resource, submitting it to the
// Python FastAPI backend and tracking the lifecycle through a phase state machine.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=bj
// +kubebuilder:printcolumn:name="GPU",type=string,JSONPath=`.spec.gpuModel`
// +kubebuilder:printcolumn:name="Workload",type=string,JSONPath=`.spec.workloadType`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Throughput",type=number,JSONPath=`.status.throughput`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type BenchmarkJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BenchmarkJobSpec   `json:"spec,omitempty"`
	Status BenchmarkJobStatus `json:"status,omitempty"`
}

// BenchmarkJobSpec defines the desired GPU benchmark workload.
type BenchmarkJobSpec struct {
	// GPUModel is the GPU model to benchmark (e.g. H100, A100).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=T4;V100;A100;H100;L40S;RTX4090
	GPUModel string `json:"gpuModel"`

	// WorkloadType is the benchmark workload class.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=inference;training
	WorkloadType string `json:"workloadType"`

	// BatchSize is the batch dimension for the benchmark run.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1024
	BatchSize int `json:"batchSize"`

	// SchedulerAPIURL is the base URL of the Python benchmark API service.
	// +kubebuilder:default="http://gpu-benchmark-api.gpu-benchmark.svc.cluster.local:8000"
	SchedulerAPIURL string `json:"schedulerAPIURL,omitempty"`

	// MaxRetries is the maximum number of times a failed job is retried before
	// transitioning to Failed.
	// +kubebuilder:default=3
	MaxRetries int `json:"maxRetries,omitempty"`
}

// BenchmarkJobStatus describes the observed state of a BenchmarkJob.
type BenchmarkJobStatus struct {
	// Phase is the current lifecycle phase of the benchmark job.
	// +kubebuilder:validation:Enum=Pending;Submitted;Running;Completed;Failed
	Phase string `json:"phase,omitempty"`

	// JobID is the UUID assigned by the Python API backend upon submission.
	JobID string `json:"jobID,omitempty"`

	// Message is a human-readable explanation of the current phase.
	Message string `json:"message,omitempty"`

	// RetryCount tracks how many times submission or execution has been retried.
	RetryCount int `json:"retryCount,omitempty"`

	// Throughput is the measured samples/second from the completed benchmark.
	Throughput *float64 `json:"throughput,omitempty"`

	// LatencyMs is the measured inference/training latency in milliseconds.
	LatencyMs *float64 `json:"latencyMs,omitempty"`

	// MemoryBandwidthGbps is the measured GPU memory bandwidth in GB/s.
	MemoryBandwidthGbps *float64 `json:"memoryBandwidthGbps,omitempty"`

	// StartTime records when the job transitioned to Submitted.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime records when the job reached a terminal phase.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

// +kubebuilder:object:root=true

// BenchmarkJobList contains a list of BenchmarkJob resources.
type BenchmarkJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BenchmarkJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BenchmarkJob{}, &BenchmarkJobList{})
}
