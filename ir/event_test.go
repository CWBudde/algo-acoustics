package ir

import "testing"

func TestEventKindOrdering(t *testing.T) {
	tests := []struct {
		name string
		kind EventKind
		want int
	}{
		{name: "direct", kind: EventDirect, want: 0},
		{name: "specular", kind: EventSpecular, want: 1},
		{name: "diffuse", kind: EventDiffuse, want: 2},
		{name: "diffraction", kind: EventDiffraction, want: 3},
		{name: "pde", kind: EventPDE, want: 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := int(tc.kind); got != tc.want {
				t.Fatalf("kind = %d, want %d", got, tc.want)
			}
		})
	}
}
