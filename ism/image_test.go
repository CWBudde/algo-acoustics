package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestGenerateImageSourcesFirstOrder(t *testing.T) {
	t.Parallel()

	room := &scene.Shoebox{Width: 10, Depth: 8, Height: 6}
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}
	imageSources := GenerateImageSources(src, room, 1)

	if len(imageSources) != 7 {
		t.Fatalf("GenerateImageSources() returned %d image sources, want 7", len(imageSources))
	}

	wantByPosition := map[geometry.Vec3]ImageSource{
		{X: 1, Y: 1, Z: 1}:  {Order: 0, WallMask: 0},
		{X: -1, Y: 1, Z: 1}: {Order: 1, WallMask: wallBitNegX},
		{X: 19, Y: 1, Z: 1}: {Order: 1, WallMask: wallBitPosX},
		{X: 1, Y: -1, Z: 1}: {Order: 1, WallMask: wallBitNegY},
		{X: 1, Y: 15, Z: 1}: {Order: 1, WallMask: wallBitPosY},
		{X: 1, Y: 1, Z: -1}: {Order: 1, WallMask: wallBitNegZ},
		{X: 1, Y: 1, Z: 11}: {Order: 1, WallMask: wallBitPosZ},
	}

	for _, img := range imageSources {
		want, ok := wantByPosition[img.Position]
		if !ok {
			t.Fatalf("unexpected image source position: %+v", img.Position)
		}
		if img.Order != want.Order {
			t.Fatalf("image source %+v order = %d, want %d", img.Position, img.Order, want.Order)
		}
		if img.WallMask != want.WallMask {
			t.Fatalf("image source %+v wall mask = %06b, want %06b", img.Position, img.WallMask, want.WallMask)
		}
	}
}

func TestIsAudibleRejectsCornerHit(t *testing.T) {
	t.Parallel()

	room := &scene.Shoebox{Width: 4, Depth: 4, Height: 4}
	src := geometry.Vec3{X: 1, Y: 1, Z: 1}
	imageSources := GenerateImageSources(src, room, 2)

	var cornerImage ImageSource
	found := false
	for _, img := range imageSources {
		if img.orderX == -1 && img.orderY == -1 && img.orderZ == 0 {
			cornerImage = img
			found = true
			break
		}
	}
	if !found {
		t.Fatal("failed to find the expected corner-grazing image source")
	}

	receiver := geometry.Vec3{X: 3, Y: 3, Z: 1}
	if IsAudible(cornerImage, receiver) {
		t.Fatal("IsAudible() = true, want false for a corner-grazing path")
	}
}
