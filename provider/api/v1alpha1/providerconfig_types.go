package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderConfig configures the GPU Benchmark Provider by pointing it at the
// Python FastAPI scheduler API. Teams create a ProviderConfig once per cluster
// and reference it from their GPUBenchmarkJob Managed Resources via
// spec.providerConfigRef.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="SECRET-NAME",type=string,JSONPath=`.spec.credentials.secretRef.name`,priority=1
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// ProviderConfigSpec defines the configuration for the GPU Benchmark Provider.
type ProviderConfigSpec struct {
	// SchedulerAPIURL is the base URL of the GPU benchmark Python FastAPI service.
	// The provider will POST to <schedulerAPIURL>/benchmarks to create jobs and
	// GET/DELETE <schedulerAPIURL>/benchmarks/{id} to observe and delete them.
	//
	// Defaults to the in-cluster service URL when running inside Kubernetes.
	// +kubebuilder:default="http://gpu-benchmark-api.gpu-benchmark.svc.cluster.local:8000"
	SchedulerAPIURL string `json:"schedulerAPIURL"`

	// PollInterval controls how often the provider polls the backend for status
	// updates on in-progress benchmark jobs (default: 15s).
	// +kubebuilder:default="15s"
	PollInterval metav1.Duration `json:"pollInterval,omitempty"`
}

// ProviderConfigStatus shows the observed state of the ProviderConfig.
type ProviderConfigStatus struct {
	// ProviderConfigStatus contains the standard Crossplane usage tracking fields:
	// the number of Managed Resources currently using this ProviderConfig.
	xpv1.ProviderConfigStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig objects.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// +kubebuilder:object:root=true

// ProviderConfigUsage indicates that a resource is using a ProviderConfig.
// Crossplane uses these objects to track how many Managed Resources depend
// on a given ProviderConfig, preventing premature deletion.
//
// +kubebuilder:resource:scope=Cluster,categories=crossplane
type ProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpv1.ProviderConfigUsage `json:",inline"`
}

// +kubebuilder:object:root=true

// ProviderConfigUsageList contains a list of ProviderConfigUsage objects.
type ProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfigUsage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
	SchemeBuilder.Register(&ProviderConfigUsage{}, &ProviderConfigUsageList{})
}
