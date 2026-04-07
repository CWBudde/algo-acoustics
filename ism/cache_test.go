package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestShoeboxCache_ValidFor(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	sources := GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, 2)
	cache := NewShoeboxCache(sources, &sc)

	if !cache.ValidFor(&sc) {
		t.Fatal("cache should be valid for the same scene")
	}
}

func TestShoeboxCache_InvalidAfterGeometryChange(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	sources := GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, 2)
	cache := NewShoeboxCache(sources, &sc)

	// Change room width.
	modified := sc
	modified.Room.Shoebox = &scene.Shoebox{
		Width:         sc.Room.Shoebox.Width + 1,
		Depth:         sc.Room.Shoebox.Depth,
		Height:        sc.Room.Shoebox.Height,
		WallMaterials: sc.Room.Shoebox.WallMaterials,
	}

	if cache.ValidFor(&modified) {
		t.Fatal("cache should be invalid after room width change")
	}
}

func TestShoeboxCache_ValidAfterMaterialChange(t *testing.T) {
	t.Parallel()

	sc := testScene(t)
	sources := GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, 2)
	cache := NewShoeboxCache(sources, &sc)

	// Change material absorption.
	modified := sc
	modified.Materials = map[string]scene.Material{
		"hard": {
			Name:             "hard",
			AbsorptionByBand: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
			ScatteringByBand: []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		},
	}

	if !cache.ValidFor(&modified) {
		t.Fatal("cache should remain valid after material change (geometry unchanged)")
	}
}

func TestMeshCache_ValidFor(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	imgCfg := MeshISMConfig{
		MaxOrder:    2,
		MaxDistance: meshMaxDistance(sc, acoustics.SpeedOfSound),
	}

	sources := GenerateMeshImageSources(sc.Sources[0].Position, sc.Room.Mesh, imgCfg)
	cache := NewMeshCache(sources, sc)

	if !cache.ValidFor(sc) {
		t.Fatal("cache should be valid for the same scene")
	}
}

func TestMeshCache_InvalidAfterGeometryChange(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	imgCfg := MeshISMConfig{
		MaxOrder:    2,
		MaxDistance: meshMaxDistance(sc, acoustics.SpeedOfSound),
	}

	sources := GenerateMeshImageSources(sc.Sources[0].Position, sc.Room.Mesh, imgCfg)
	cache := NewMeshCache(sources, sc)

	// Change mesh geometry.
	modified := *sc
	modified.Room.Mesh = geometry.MeshFromBox(geometry.Vec3{}, geometry.Vec3{X: 5, Y: 4, Z: 3})

	if cache.ValidFor(&modified) {
		t.Fatal("cache should be invalid after mesh geometry change")
	}
}

func TestMeshCache_ValidAfterMaterialChange(t *testing.T) {
	t.Parallel()

	sc := testMeshScene()
	imgCfg := MeshISMConfig{
		MaxOrder:    2,
		MaxDistance: meshMaxDistance(sc, acoustics.SpeedOfSound),
	}

	sources := GenerateMeshImageSources(sc.Sources[0].Position, sc.Room.Mesh, imgCfg)
	cache := NewMeshCache(sources, sc)

	// Change material.
	modified := *sc
	modified.Materials = map[string]scene.Material{
		"plaster": {
			Name:             "plaster",
			AbsorptionByBand: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
		},
	}

	if !cache.ValidFor(&modified) {
		t.Fatal("cache should remain valid after material change")
	}
}
