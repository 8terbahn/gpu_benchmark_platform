// Package v1alpha1 contains the v1alpha1 API types for the GPU Benchmark Provider.
//
// The provider manages GPUBenchmarkJob Managed Resources, which correspond to
// benchmark jobs executed by the Python FastAPI backend. A ProviderConfig
// points the provider at the target scheduler API endpoint.
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package-level variables for the API group, version, and registered types.
var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "gpu.platform.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add functions to this scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme

	// GPUBenchmarkJobGroupVersionKind is the GroupVersionKind for GPUBenchmarkJob.
	// Used by the managed reconciler to identify the resource kind.
	GPUBenchmarkJobGroupVersionKind = schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    "GPUBenchmarkJob",
	}
)
