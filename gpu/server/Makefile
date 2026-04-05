# Makefile — GPU server binary
#
# Builds algo-acoustics-gpu from server.cu + the shared FDTD kernel.
# Requires CUDA Toolkit (nvcc) and a sm_75+ GPU (RTX series / Quadro T-series).
#
# Usage:
#   make          → build algo-acoustics-gpu
#   make clean    → remove build artefacts
#   make install  → copy binary to ../../bin/

NVCC      ?= nvcc
ARCH      := sm_75
CXXSTD    := c++17
NVCCFLAGS := -arch=$(ARCH) -std=$(CXXSTD) -O3 \
             -Xcompiler -Wall,-Wextra

TARGET    := algo-acoustics-gpu
SRCS      := server.cu ../fdtd/fdtd_kernel.cu ../raytrace/bvh_kernel.cu

.PHONY: all clean install

all: $(TARGET)

$(TARGET): $(SRCS) protocol.h ../fdtd/fdtd_kernel.cuh \
           ../raytrace/bvh.h ../raytrace/bvh_kernel.h
	$(NVCC) $(NVCCFLAGS) -o $@ $(SRCS)

install: $(TARGET)
	mkdir -p ../../bin
	cp $(TARGET) ../../bin/

clean:
	rm -f $(TARGET)
