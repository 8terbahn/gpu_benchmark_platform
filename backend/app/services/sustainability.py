"""
Carbon footprint calculation service for GPU workloads.

GPU TDP values are sourced from manufacturer specifications (Watts).
Regional grid carbon intensity values are in gCO2eq/kWh.
"""

from pydantic import BaseModel, Field

# ---------------------------------------------------------------------------
# Mock reference data
# ---------------------------------------------------------------------------

# GPU Thermal Design Power in Watts
GPU_TDP_WATTS: dict[str, float] = {
    "H100": 700.0,
    "A100": 400.0,
    "A10G": 150.0,
    "V100": 300.0,
    "T4": 70.0,
    "RTX4090": 450.0,
    "RTX3090": 350.0,
    "L40S": 350.0,
    "A6000": 300.0,
}

# Regional grid carbon intensity in gCO2eq/kWh
REGION_CARBON_INTENSITY_G_CO2EQ_PER_KWH: dict[str, float] = {
    "eu-central": 300.0,
    "eu-west": 250.0,
    "us-east": 400.0,
    "us-west": 150.0,
    "us-central": 450.0,
    "ap-southeast": 500.0,
    "ap-northeast": 350.0,
    "ca-central": 120.0,
    "sa-east": 200.0,
}


# ---------------------------------------------------------------------------
# Response model
# ---------------------------------------------------------------------------


class CarbonFootprintReport(BaseModel):
    gpu_model: str = Field(..., description="GPU model identifier")
    region: str = Field(..., description="Cloud / data-centre region")
    runtime_hours: float = Field(..., ge=0, description="Total GPU runtime in hours")
    gpu_tdp_watts: float = Field(..., description="GPU Thermal Design Power in Watts")
    grid_carbon_intensity_g_co2eq_per_kwh: float = Field(
        ..., description="Grid carbon intensity for the region in gCO2eq/kWh"
    )
    energy_consumed_kwh: float = Field(..., description="Total energy consumed in kWh")
    carbon_emitted_kg_co2eq: float = Field(
        ..., description="Total carbon emissions in kg CO2-equivalent"
    )


# ---------------------------------------------------------------------------
# Service function
# ---------------------------------------------------------------------------


def calculate_carbon_footprint(
    gpu_model: str,
    runtime_hours: float,
    region: str,
) -> CarbonFootprintReport:
    """Calculate the energy consumption and carbon footprint of a GPU workload.

    Args:
        gpu_model: GPU model identifier (e.g. ``"H100"``).  A case-insensitive
            lookup is performed; unknown models raise :class:`ValueError`.
        runtime_hours: Duration the GPU ran, in hours.  Must be >= 0.
        region: Data-centre region key (e.g. ``"us-east"``).  An unknown region
            raises :class:`ValueError`.

    Returns:
        :class:`CarbonFootprintReport` containing energy and carbon figures.

    Raises:
        ValueError: If *gpu_model* or *region* are not found in the reference
            dictionaries.
        ValueError: If *runtime_hours* is negative.
    """
    if runtime_hours < 0:
        raise ValueError(f"runtime_hours must be >= 0, got {runtime_hours}")

    # Normalise keys for forgiving look-ups
    normalised_gpu = gpu_model.upper().replace(" ", "")
    tdp = GPU_TDP_WATTS.get(normalised_gpu)
    if tdp is None:
        available = ", ".join(sorted(GPU_TDP_WATTS))
        raise ValueError(
            f"Unknown GPU model '{gpu_model}'. Available models: {available}"
        )

    normalised_region = region.lower().strip()
    carbon_intensity = REGION_CARBON_INTENSITY_G_CO2EQ_PER_KWH.get(normalised_region)
    if carbon_intensity is None:
        available = ", ".join(sorted(REGION_CARBON_INTENSITY_G_CO2EQ_PER_KWH))
        raise ValueError(
            f"Unknown region '{region}'. Available regions: {available}"
        )

    # Energy (kWh) = Power (W) / 1000 * Hours
    energy_kwh = (tdp / 1_000.0) * runtime_hours

    # Carbon (kg CO2eq) = Energy (kWh) * Intensity (gCO2eq/kWh) / 1000
    carbon_kg = energy_kwh * carbon_intensity / 1_000.0

    return CarbonFootprintReport(
        gpu_model=gpu_model,
        region=region,
        runtime_hours=runtime_hours,
        gpu_tdp_watts=tdp,
        grid_carbon_intensity_g_co2eq_per_kwh=carbon_intensity,
        energy_consumed_kwh=round(energy_kwh, 6),
        carbon_emitted_kg_co2eq=round(carbon_kg, 6),
    )
