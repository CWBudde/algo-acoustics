# gpu/fdtd/Makefile
#
# Builds the FDTD stencil benchmark for the installed GPU.
# Requires: nvcc (CUDA Toolkit 11+), C++17 capable host compiler.
#
# Usage:
#   make                 # build for auto-detected SM arch
#   make SM_ARCH=sm_75   # build for specific arch (T550 = sm_75)
#   make run             # build and run benchmark
#   make profile         # run under Nsight Compute (ncu)

SM_ARCH ?= $(shell nvcc --run -x cu - <<< "" 2>&1 | grep "arch" | head -1 || echo sm_75)
# Fallback: detect via nvidia-smi
ifeq ($(SM_ARCH),sm_75)
SM_ARCH := $(shell nvidia-smi --query-gpu=compute_cap --format=csv,noheader 2>/dev/null | head -1 | tr -d '.' | xargs -I{} echo sm_{} || echo sm_75)
endif

NVCC     := nvcc
CXXFLAGS := -std=c++17 -O3 -lineinfo
NVFLAGS  := -arch=$(SM_ARCH) $(CXXFLAGS) --generate-line-info
TARGET   := fdtd_bench

SRCS := fdtd_bench.cu fdtd_kernel.cu

.PHONY: all run profile clean

all: $(TARGET)

$(TARGET): $(SRCS) fdtd_kernel.cuh
	$(NVCC) $(NVFLAGS) -o $@ $(SRCS)
	@echo "Built $(TARGET) for $(SM_ARCH)"

run: $(TARGET)
	./$(TARGET)

# Nsight Compute profile (requires ncu from CUDA Toolkit).
# Profiles the two most interesting kernels on a 256³ grid.
profile: $(TARGET)
	ncu --set full -k "fdtd_step_naive|fdtd_step_tiled" \
	    --csv ./$(TARGET) > fdtd_ncu.csv 2>&1 || \
	ncu --set full -k regex:"fdtd_step" ./$(TARGET)

clean:
	rm -f $(TARGET) *.o fdtd_ncu.csv
