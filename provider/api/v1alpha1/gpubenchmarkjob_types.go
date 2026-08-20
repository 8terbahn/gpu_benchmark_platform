package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUBenchmarkJob is a Crossplane Managed Resource representing a GPU benchmark
// workload executed by the Python FastAPI backend.
//
// Teams declare a GPUBenchmarkJob to request benchmark execution. The provider
// reconciles the desired state (spec.forProvider) against the external API,
// driving the job through its lifecycle and writing results to status.atProvider.
//
// This is the Crossplane-native counterpart to the BenchmarkJob CRD managed by
// the Kubernetes Operator; both coexist and demonstrate different control-plane
// patterns. The Provider pattern is preferred when the lifecycle of the external
// resource should be owned entirely by Crossplane's managed reconciler.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=gbj
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="SYNCED",type=string,JSONPath=`.status.conditions[?(@.type=='Synced')].status`
// +kubebuilder:printcolumn:name="GPU",type=string,JSONPath=`.spec.forProvider.gpuModel`
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=`.status.atProvider.phase`
// +kubebuilder:printcolumn:name="THROUGHPUT",type=number,JSONPath=`.status.atProvider.throughput`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
type GPUBenchmarkJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUBenchmarkJobSpec   `json:"spec"`
	Status GPUBenchmarkJobStatus `json:"status,omitempty"`
}

// GPUBenchmarkJobSpec defines the desired state of a GPUBenchmarkJob.
type GPUBenchmarkJobSpec struct {
	// ResourceSpec contains standard Crossplane Managed Resource fields:
	// providerConfigRef, deletionPolicy, managementPolicies, etc.
	xpv1.ResourceSpec `json:",inline"`

	// ForProvider describes the parameters of the external benchmark resource.
	// These fields map directly to the Python FastAPI POST /benchmarks payload.
	ForProvider GPUBenchmarkJobParameters `json:"forProvider"`
}

// GPUBenchmarkJobParameters describes the configurable fields of the external
// GPU benchmark resource. These are reconciled against the Python API backend.
type GPUBenchmarkJobParameters struct {
	// GPUModel is the GPU model to benchmark.
	// +kubebuilder:validation:Enum=T4;V100;A100;H100;L40S;RTX4090
	GPUModel string `json:"gpuModel"`

	// WorkloadType is the benchmark workload class.
	// +kubebuilder:validation:Enum=inference;training
	WorkloadType string `json:"workloadType"`

	// BatchSize is the batch dimension for the benchmark run.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1024
	BatchSize int `json:"batchSize"`

	// MaxRetries is the maximum number of times the provider will re-submit a
	// failed job before marking the Managed Resource as unavailable.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	MaxRetries int `json:"maxRetries,omitempty"`
}

// GPUBenchmarkJobStatus defines the observed state of a GPUBenchmarkJob.
type GPUBenchmarkJobStatus struct {
	// ConditionedStatus surfaces Ready and Synced conditions following the
	// standard Crossplane Managed Resource status contract.
	xpv1.ConditionedStatus `json:",inline"`

	// AtProvider contains the observed state of the external benchmark resource.
	// Fields here are populated by the provider's Observe loop.
	AtProvider GPUBenchmarkJobObservation `json:"atProvider,omitempty"`
}

// GPUBenchmarkJobObservation contains the fields observed from the external
// Python API — i.e., state that exists in the backend but was not specified
// by the user in spec.forProvider.
type GPUBenchmarkJobObservation struct {
	// JobID is the UUID assigned by the Python API backend upon job creation.
	JobID string `json:"jobID,omitempty"`

	// Phase is the current lifecycle phase reported by the backend.
	// One of: queued, running, completed, failed.
	Phase string `json:"phase,omitempty"`

	// RetryCount is the number of times the provider has re-submitted this job
	// after a backend failure.
	RetryCount int `json:"retryCount,omitempty"`

	// Throughput is the measured samples/second from the completed benchmark.
	Throughput *float64 `json:"throughput,omitempty"`

	// LatencyMs is the measured inference/training latency in milliseconds.
	LatencyMs *float64 `json:"latencyMs,omitempty"`

	// MemoryBandwidthGbps is the measured GPU memory bandwidth in GB/s.
	MemoryBandwidthGbps *float64 `json:"memoryBandwidthGbps,omitempty"`
}

// +kubebuilder:object:root=true

// GPUBenchmarkJobList contains a list of GPUBenchmarkJob Managed Resources.
type GPUBenchmarkJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUBenchmarkJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPUBenchmarkJob{}, &GPUBenchmarkJobList{})
}
