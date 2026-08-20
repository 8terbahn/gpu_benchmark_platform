// Package v1alpha1 contains API types for the GPU Benchmark Platform operator.
//
// The API group is benchmarks.gpu-platform.io.
// Teams submit BenchmarkJob CRs to declaratively request GPU benchmark execution.
// The operator reconciles these CRs against the Python FastAPI backend.
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group and version for this API.
	GroupVersion = schema.GroupVersion{Group: "benchmarks.gpu-platform.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add the types in this package to a scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this package to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
