//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
	"testing"
)

// The page sizes its sliders from this object (web/demo-limits.mjs), so both the
// key names and the values are a contract, not an implementation detail.
func TestDemoLimitsToJSPublishesTheEnvelope(t *testing.T) {
	t.Parallel()

	limits := demoLimitsToJS()

	tests := []struct {
		key  string
		want float64
	}{
		{key: "sampleRate", want: demoSampleRate},
		{key: "maxSurfaces", want: maxDemoSurfaces},
		{key: "maxMeshTriangles", want: maxDemoMeshTriangles},
		{key: "maxMaterials", want: maxDemoMaterials},
		{key: "minNumRays", want: minDemoNumRays},
		{key: "maxNumRays", want: maxDemoNumRays},
		{key: "minMaxOrder", want: minDemoMaxOrder},
		{key: "maxMaxOrder", want: maxDemoMaxOrder},
		{key: "minDurationSeconds", want: minDemoDurationSecs},
		{key: "maxDurationSeconds", want: maxDemoDurationSecs},
		{key: "minRoomMeters", want: minDemoRoomMeters},
		{key: "maxRoomMeters", want: maxDemoRoomMeters},
		{key: "renderTimeoutSeconds", want: demoRenderTimeout.Seconds()},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			value := limits.Get(tt.key)
			if value.Type() != js.TypeNumber {
				t.Fatalf("limits.%s is %v, want a number", tt.key, value.Type())
			}

			if got := value.Float(); got != tt.want {
				t.Errorf("limits.%s = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// subdividedWallMesh returns a single flat wall cut into 2*n² triangles. It is
// the case the surface limit must not be fooled by: many triangles, one plane.
func subdividedWallMesh(n int) *demoMesh {
	mesh := &demoMesh{Triangles: make([]demoTriangle, 0, 2*n*n)}
	step := 4.0 / float64(n)

	for row := range n {
		for column := range n {
			y0 := float64(column) * step
			z0 := float64(row) * step
			y1 := y0 + step
			z1 := z0 + step

			mesh.Triangles = append(
				mesh.Triangles,
				demoTriangle{
					V0: demoPoint{Y: y0, Z: z0},
					V1: demoPoint{Y: y1, Z: z0},
					V2: demoPoint{Y: y1, Z: z1},
				},
				demoTriangle{
					V0: demoPoint{Y: y0, Z: z0},
					V1: demoPoint{Y: y1, Z: z1},
					V2: demoPoint{Y: y0, Z: z1},
				},
			)
		}
	}

	return mesh
}

// staircaseMesh returns count triangles that each lie in their own plane.
func staircaseMesh(count int) *demoMesh {
	mesh := &demoMesh{Triangles: make([]demoTriangle, 0, count)}

	for index := range count {
		offset := float64(index) * 0.37

		mesh.Triangles = append(mesh.Triangles, demoTriangle{
			V0: demoPoint{X: offset},
			V1: demoPoint{X: offset + 1, Y: 1},
			V2: demoPoint{X: offset, Y: 0.5, Z: 1 + float64(index)},
		})
	}

	return mesh
}

func TestCountDemoSurfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		room demoRoom
		want int
	}{
		{
			name: "shoebox has six walls",
			room: demoRoom{Kind: "shoebox", Width: 6, Depth: 4, Height: 3},
			want: 6,
		},
		{
			name: "mesh room without geometry falls back to the shoebox count",
			room: demoRoom{Kind: "mesh"},
			want: 6,
		},
		{
			name: "subdivision does not add surfaces",
			room: demoRoom{Kind: "mesh", Mesh: subdividedWallMesh(10)},
			want: 1,
		},
		{
			name: "distinct planes are counted separately",
			room: demoRoom{Kind: "mesh", Mesh: staircaseMesh(7)},
			want: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := countDemoSurfaces(tt.room); got != tt.want {
				t.Errorf("countDemoSurfaces() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountDemoSurfacesIgnoresBackFacesAndDegenerates(t *testing.T) {
	t.Parallel()

	// The same plane wound both ways, plus a zero-area triangle.
	mesh := &demoMesh{Triangles: []demoTriangle{
		{V0: demoPoint{}, V1: demoPoint{X: 1}, V2: demoPoint{Y: 1}},
		{V0: demoPoint{}, V1: demoPoint{Y: 1}, V2: demoPoint{X: 1}},
		{V0: demoPoint{}, V1: demoPoint{X: 1}, V2: demoPoint{X: 2}},
	}}

	if got := countDemoSurfaces(demoRoom{Kind: "mesh", Mesh: mesh}); got != 1 {
		t.Errorf("countDemoSurfaces() = %d, want 1", got)
	}
}

func TestValidateDemoStructureRejectsTooManySurfaces(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Room.Kind = "mesh"
	request.Room.Mesh = staircaseMesh(maxDemoSurfaces + 1)

	err := validateDemoStructure(request)
	if err == nil {
		t.Fatal("validateDemoStructure() error = nil, want a surface-limit error")
	}

	if !strings.Contains(err.Error(), "distinct surfaces") {
		t.Errorf("error = %q, want it to name the surface limit", err)
	}
}

func TestValidateDemoStructureAcceptsSubdividedGeometry(t *testing.T) {
	t.Parallel()

	// 2 * 40² = 3,200 triangles in one plane: far past the surface limit if
	// triangles were counted, comfortably inside it when planes are.
	request := defaultDemoRequest()
	request.Room.Kind = "mesh"
	request.Room.Mesh = subdividedWallMesh(40)

	err := validateDemoStructure(request)
	if err != nil {
		t.Fatalf("validateDemoStructure() error = %v, want nil", err)
	}
}

func TestNormalizeDemoRequestEnforcesTheQualityEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mutate       func(*demoRequest)
		wantRays     int
		wantOrder    int
		wantDuration float64
	}{
		{
			name: "above the envelope",
			mutate: func(request *demoRequest) {
				request.Render.NumRays = maxDemoNumRays * 8
				request.Render.MaxOrder = maxDemoMaxOrder + 5
				request.Render.DurationSeconds = maxDemoDurationSecs + 7
			},
			wantRays:     maxDemoNumRays,
			wantOrder:    maxDemoMaxOrder,
			wantDuration: maxDemoDurationSecs,
		},
		{
			name: "below the envelope",
			mutate: func(request *demoRequest) {
				request.Render.NumRays = 1
				request.Render.MaxOrder = -3
				request.Render.DurationSeconds = 0.001
			},
			wantRays:     minDemoNumRays,
			wantOrder:    defaultDemoMaxOrder,
			wantDuration: minDemoDurationSecs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := defaultDemoRequest()
			tt.mutate(&request)

			normalized, _, err := normalizeDemoRequest(request)
			if err != nil {
				t.Fatalf("normalizeDemoRequest() error = %v", err)
			}

			if normalized.Render.NumRays != tt.wantRays {
				t.Errorf("NumRays = %d, want %d", normalized.Render.NumRays, tt.wantRays)
			}

			if normalized.Render.MaxOrder != tt.wantOrder {
				t.Errorf("MaxOrder = %d, want %d", normalized.Render.MaxOrder, tt.wantOrder)
			}

			if normalized.Render.DurationSeconds != tt.wantDuration {
				t.Errorf("DurationSeconds = %v, want %v", normalized.Render.DurationSeconds, tt.wantDuration)
			}
		})
	}
}

// The plan's headline limit is an impulse response of up to three seconds at
// 48 kHz, which is 144,000 samples.
func TestMaximumDurationRendersTheFullSampleCount(t *testing.T) {
	t.Parallel()

	request := defaultDemoRequest()
	request.Render.Mode = "early"
	request.Render.MaxOrder = 1
	request.Render.NumRays = minDemoNumRays
	request.Render.DurationSeconds = maxDemoDurationSecs

	result, err := runDemoRender(request)
	if err != nil {
		t.Fatalf("runDemoRender() error = %v", err)
	}

	if want := int(maxDemoDurationSecs * demoSampleRate); len(result.Samples) != want {
		t.Errorf("sample count = %d, want %d", len(result.Samples), want)
	}

	if result.SampleRate != demoSampleRate {
		t.Errorf("SampleRate = %d, want %d", result.SampleRate, demoSampleRate)
	}
}
