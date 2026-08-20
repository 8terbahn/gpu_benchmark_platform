// Package main implements a Crossplane Composition Function for GPU benchmark sizing.
//
// A Crossplane Function runs as a gRPC server inside a container. The Composition
// Engine calls RunFunction for each pipeline step, passing the observed composite
// resource state and expecting a desired state in return.
//
// This function:
//  1. Reads the XBenchmarkEnvironment spec (gpuModel, workloadType, batchSize).
//  2. Validates gpuModel and workloadType against known enumerations.
//  3. Estimates benchmark runtime based on GPU tier and workload class.
//  4. Annotates the composite resource with computed values for downstream steps.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// gpuBenchmarkTier defines per-GPU-type benchmark performance characteristics.
type gpuBenchmarkTier struct {
	estimatedRuntimeSec int     // estimated benchmark duration in seconds
	peakThroughput      float64 // approximate peak samples/sec for inference
	memoryBandwidthGbps float64 // approximate peak memory bandwidth in GB/s
}

// tierTable maps GPU model → benchmark sizing parameters.
var tierTable = map[string]gpuBenchmarkTier{
	"T4":     {estimatedRuntimeSec: 120, peakThroughput: 400, memoryBandwidthGbps: 300},
	"V100":   {estimatedRuntimeSec: 90, peakThroughput: 900, memoryBandwidthGbps: 900},
	"A100":   {estimatedRuntimeSec: 60, peakThroughput: 2000, memoryBandwidthGbps: 2000},
	"H100":   {estimatedRuntimeSec: 30, peakThroughput: 3500, memoryBandwidthGbps: 3350},
	"L40S":   {estimatedRuntimeSec: 45, peakThroughput: 1800, memoryBandwidthGbps: 864},
	"RTX4090":{estimatedRuntimeSec: 75, peakThroughput: 1200, memoryBandwidthGbps: 1008},
}

// validWorkloads is the set of accepted workload types.
var validWorkloads = map[string]bool{"inference": true, "training": true}

// FunctionRunner implements the Crossplane FunctionRunnerService gRPC interface.
type FunctionRunner struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer
	log *slog.Logger
}

func (f *FunctionRunner) RunFunction(
	_ context.Context,
	req *fnv1beta1.RunFunctionRequest,
) (*fnv1beta1.RunFunctionResponse, error) {
	xrName := req.GetObserved().GetComposite().GetResource().GetMetadata().GetName()
	f.log.Info("RunFunction called", "xr", xrName)

	specFields := req.GetObserved().GetComposite().GetResource().GetSpec().GetFields()

	gpuModel    := strField(specFields, "gpuModel", "")
	workload    := strField(specFields, "workloadType", "")
	batchSize   := int(numField(specFields, "batchSize", 32))
	costCenter  := strField(specFields, "costCenter", "CC-0000")

	// Validate GPU model.
	tier, ok := tierTable[gpuModel]
	if !ok {
		return fatalResponse(fmt.Sprintf(
			"unsupported gpuModel %q; allowed: T4, V100, A100, H100, L40S, RTX4090", gpuModel))
	}

	// Validate workload type.
	if !validWorkloads[workload] {
		return fatalResponse(fmt.Sprintf(
			"unsupported workloadType %q; allowed: inference, training", workload))
	}

	// Compute estimated runtime — training takes ~3x longer than inference.
	runtimeMultiplier := 1.0
	if workload == "training" {
		runtimeMultiplier = 3.0
	}
	// Batch size > 64 adds overhead proportional to extra memory transfers.
	batchMultiplier := 1.0
	if batchSize > 64 {
		batchMultiplier = 1.0 + float64(batchSize-64)/256.0
	}
	estimatedSec := int(float64(tier.estimatedRuntimeSec) * runtimeMultiplier * batchMultiplier)

	f.log.Info("benchmark sizing computed",
		"gpuModel", gpuModel,
		"workloadType", workload,
		"batchSize", batchSize,
		"costCenter", costCenter,
		"estimatedRuntimeSec", estimatedSec,
		"peakThroughput", tier.peakThroughput,
	)

	// Annotate the composite resource with sizing metadata.
	resp := &fnv1beta1.RunFunctionResponse{
		Desired: req.GetObserved(),
		Results: []*fnv1beta1.Result{{
			Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
			Message: fmt.Sprintf(
				"benchmark sizing: %s/%s batch=%d est=%ds",
				gpuModel, workload, batchSize, estimatedSec),
		}},
	}

	annotations := resp.GetDesired().GetComposite().GetResource().GetMetadata().GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["platform.gpu-platform.io/estimated-runtime-sec"] = fmt.Sprintf("%d", estimatedSec)
	annotations["platform.gpu-platform.io/peak-throughput"]        = fmt.Sprintf("%.0f", tier.peakThroughput)
	annotations["platform.gpu-platform.io/cost-center"]            = costCenter

	return resp, nil
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := ":9443"
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("listen failed", "error", err)
		os.Exit(1)
	}
	srv := grpc.NewServer()
	fnv1beta1.RegisterFunctionRunnerServiceServer(srv, &FunctionRunner{log: log})
	reflection.Register(srv)
	log.Info("Benchmark Sizing Function started", "addr", addr)
	if err := srv.Serve(lis); err != nil {
		log.Error("gRPC server error", "error", err)
		os.Exit(1)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func strField(fields map[string]interface{}, key, def string) string {
	if v, ok := fields[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func numField(fields map[string]interface{}, key string, def float64) float64 {
	if v, ok := fields[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return def
}

func fatalResponse(msg string) (*fnv1beta1.RunFunctionResponse, error) {
	return &fnv1beta1.RunFunctionResponse{
		Results: []*fnv1beta1.Result{{
			Severity: fnv1beta1.Severity_SEVERITY_FATAL,
			Message:  msg,
		}},
	}, nil
}
