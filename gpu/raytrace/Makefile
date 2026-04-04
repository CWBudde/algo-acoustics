# gpu/raytrace/Makefile
#
# Builds the ray-BVH traversal benchmark.
# Requires: nvcc (CUDA Toolkit 11+), C++17 host compiler.
#
# OptiX variant: not built by default (OptiX SDK not installed).
# To enable: install OptiX SDK, set OPTIX_DIR, and run: make optix

SM_ARCH  ?= sm_75
NVCC     := nvcc
CXXFLAGS := -std=c++17 -O3 -lineinfo
NVFLAGS  := -arch=$(SM_ARCH) $(CXXFLAGS) --generate-line-info
TARGET   := ray_bench

SRCS := ray_bench.cu bvh_kernel.cu

.PHONY: all run profile clean

all: $(TARGET)

$(TARGET): $(SRCS) bvh.h bvh_build.h
	$(NVCC) $(NVFLAGS) -o $@ $(SRCS)
	@echo "Built $(TARGET) for $(SM_ARCH)"

run: $(TARGET)
	./$(TARGET)

profile: $(TARGET)
	ncu --set full -k trace_rays_kernel --csv ./$(TARGET) > ray_ncu.csv 2>&1 || \
	ncu --set full -k trace_rays_kernel ./$(TARGET)

# OptiX variant (requires OPTIX_DIR=/path/to/optix SDK).
# Provides hardware RT-core acceleration on Turing+.
optix:
	@echo "OptiX SDK not configured. Set OPTIX_DIR and adapt this target."
	@echo "Expected speedup: 5-20x over software BVH traversal."
	@echo "See https://developer.nvidia.com/optix for SDK download."

clean:
	rm -f $(TARGET) *.o ray_ncu.csv
