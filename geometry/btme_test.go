package geometry

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestBTMETransferFiniteAndReciprocal(t *testing.T) {
	edge := DiffractionEdge{
		Start:       Vec3{Z: -2},
		End:         Vec3{Z: 2},
		Direction:   Vec3{Z: 1},
		Length:      4,
		WedgeIndex:  1.5,
		FaceONormal: Vec3{X: 1},
	}
	source := Vec3{X: 2, Y: 0.5, Z: -0.4}
	receiver := Vec3{X: -1.5, Y: 1.25, Z: 0.7}

	forward, err := BTMETransfer(source, receiver, edge, 500, 343)
	if err != nil {
		t.Fatalf("BTMETransfer() error = %v", err)
	}

	reverse, err := BTMETransfer(receiver, source, edge, 500, 343)
	if err != nil {
		t.Fatalf("reciprocal BTMETransfer() error = %v", err)
	}

	if cmplx.Abs(forward) <= 0 || math.IsNaN(real(forward)) || math.IsInf(real(forward), 0) {
		t.Fatalf("BTMETransfer() = %v, want finite non-zero transfer", forward)
	}

	if cmplx.Abs(forward-reverse) > 1e-10 {
		t.Fatalf("BTMETransfer() reciprocity mismatch: forward=%v reverse=%v", forward, reverse)
	}
}

func TestBTMETransferRejectsInvalidInputs(t *testing.T) {
	edge := DiffractionEdge{
		Start:       Vec3{Z: -1},
		End:         Vec3{Z: 1},
		Direction:   Vec3{Z: 1},
		Length:      2,
		WedgeIndex:  1.5,
		FaceONormal: Vec3{X: 1},
	}

	_, err := BTMETransfer(Vec3{X: 1}, Vec3{X: -1}, edge, 0, 343)
	if err == nil {
		t.Fatal("BTMETransfer() error = nil, want invalid-frequency error")
	}

	_, err = BTMETransfer(Vec3{X: 1, Z: 4}, Vec3{X: -1, Z: 4}, edge, 500, 343)
	if err == nil {
		t.Fatal("BTMETransfer() error = nil, want out-of-edge Fermat error")
	}
}
