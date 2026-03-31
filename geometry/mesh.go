package geometry

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const meshDegenerateAreaEpsilon = 1e-12

// Mesh is a triangulated surface.
type Mesh struct {
	Triangles []Triangle
}

// MeshValidationIssues reports hard validation failures and softer warnings.
// Warnings still produce a non-nil error so callers can surface them.
type MeshValidationIssues struct {
	Problems []string
	Warnings []string
}

// Error formats mesh validation feedback for logs and CLI output.
func (issues *MeshValidationIssues) Error() string {
	if issues == nil {
		return ""
	}

	lines := make([]string, 0, len(issues.Problems)+len(issues.Warnings)+2)
	if len(issues.Problems) > 0 {
		lines = append(lines, "mesh validation failed:")
		for _, problem := range issues.Problems {
			lines = append(lines, "- "+problem)
		}
	}
	if len(issues.Warnings) > 0 {
		if len(issues.Problems) == 0 {
			lines = append(lines, "mesh validation warnings:")
		} else {
			lines = append(lines, "warnings:")
		}
		for _, warning := range issues.Warnings {
			lines = append(lines, "- "+warning)
		}
	}

	return strings.Join(lines, "\n")
}

// HasProblems reports whether validation found hard failures.
func (issues *MeshValidationIssues) HasProblems() bool {
	return issues != nil && len(issues.Problems) > 0
}

// BoundingBox returns the axis-aligned bounding box of all vertices in the mesh.
// Returns a zero Box for an empty mesh.
func (m *Mesh) BoundingBox() Box {
	if m == nil || len(m.Triangles) == 0 {
		return Box{}
	}

	b := Box{
		Min: m.Triangles[0].V0,
		Max: m.Triangles[0].V0,
	}

	for _, tri := range m.Triangles {
		for _, v := range []Vec3{tri.V0, tri.V1, tri.V2} {
			b.Min = Vec3{
				X: min64(b.Min.X, v.X),
				Y: min64(b.Min.Y, v.Y),
				Z: min64(b.Min.Z, v.Z),
			}
			b.Max = Vec3{
				X: max64(b.Max.X, v.X),
				Y: max64(b.Max.Y, v.Y),
				Z: max64(b.Max.Z, v.Z),
			}
		}
	}

	return b
}

// Validate checks the mesh for degenerate triangles and reports watertightness
// warnings based on undirected edge counts.
func (m *Mesh) Validate() error {
	issues := &MeshValidationIssues{}
	if m == nil {
		issues.Problems = append(issues.Problems, "mesh is nil")
		return issues
	}
	if len(m.Triangles) == 0 {
		issues.Problems = append(issues.Problems, "mesh must contain at least one triangle")
		return issues
	}

	edgeCounts := make(map[meshEdgeKey]int, len(m.Triangles)*3)
	for index, tri := range m.Triangles {
		if tri.Area() <= meshDegenerateAreaEpsilon {
			issues.Problems = append(issues.Problems, fmt.Sprintf("triangle[%d] is degenerate", index))
			continue
		}

		edgeCounts[newMeshEdgeKey(tri.V0, tri.V1)]++
		edgeCounts[newMeshEdgeKey(tri.V1, tri.V2)]++
		edgeCounts[newMeshEdgeKey(tri.V2, tri.V0)]++
	}

	openEdges := 0
	for _, count := range edgeCounts {
		if count != 2 {
			openEdges++
		}
	}
	if openEdges > 0 {
		issues.Warnings = append(issues.Warnings, fmt.Sprintf("mesh is not watertight: %d boundary or non-manifold edges", openEdges))
	}

	if len(issues.Problems) == 0 && len(issues.Warnings) == 0 {
		return nil
	}

	return issues
}

// LoadOBJ loads a minimal triangle mesh from an OBJ file. Only vertex and face
// records are used; common metadata records are ignored.
func LoadOBJ(path string) (*Mesh, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	vertices := make([]Vec3, 0)
	triangles := make([]Triangle, 0)

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "v":
			vertex, parseErr := parseOBJVertex(fields)
			if parseErr != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber, parseErr)
			}
			vertices = append(vertices, vertex)
		case "f":
			faceTriangles, parseErr := parseOBJFace(fields, vertices)
			if parseErr != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber, parseErr)
			}
			triangles = append(triangles, faceTriangles...)
		case "vt", "vn", "o", "g", "s", "usemtl", "mtllib":
			continue
		default:
			return nil, fmt.Errorf("line %d: unsupported OBJ record %q", lineNumber, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	mesh := &Mesh{Triangles: triangles}
	if validationErr := mesh.Validate(); validationErr != nil {
		var issues *MeshValidationIssues
		if errors.As(validationErr, &issues) && !issues.HasProblems() {
			return mesh, nil
		}

		return nil, validationErr
	}

	return mesh, nil
}

type meshVertexKey struct {
	X uint64
	Y uint64
	Z uint64
}

type meshEdgeKey struct {
	A meshVertexKey
	B meshVertexKey
}

func newMeshEdgeKey(a, b Vec3) meshEdgeKey {
	keyA := newMeshVertexKey(a)
	keyB := newMeshVertexKey(b)
	if compareMeshVertexKey(keyA, keyB) <= 0 {
		return meshEdgeKey{A: keyA, B: keyB}
	}

	return meshEdgeKey{A: keyB, B: keyA}
}

func newMeshVertexKey(v Vec3) meshVertexKey {
	return meshVertexKey{
		X: math.Float64bits(v.X),
		Y: math.Float64bits(v.Y),
		Z: math.Float64bits(v.Z),
	}
}

func compareMeshVertexKey(a, b meshVertexKey) int {
	switch {
	case a.X < b.X:
		return -1
	case a.X > b.X:
		return 1
	case a.Y < b.Y:
		return -1
	case a.Y > b.Y:
		return 1
	case a.Z < b.Z:
		return -1
	case a.Z > b.Z:
		return 1
	default:
		return 0
	}
}

func parseOBJVertex(fields []string) (Vec3, error) {
	if len(fields) < 4 {
		return Vec3{}, errors.New("vertex record requires x y z coordinates")
	}

	x, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return Vec3{}, fmt.Errorf("parse vertex x: %w", err)
	}
	y, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return Vec3{}, fmt.Errorf("parse vertex y: %w", err)
	}
	z, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return Vec3{}, fmt.Errorf("parse vertex z: %w", err)
	}

	return Vec3{X: x, Y: y, Z: z}, nil
}

func parseOBJFace(fields []string, vertices []Vec3) ([]Triangle, error) {
	if len(fields) < 4 {
		return nil, errors.New("face record requires at least three vertices")
	}

	indices := make([]int, 0, len(fields)-1)
	for _, field := range fields[1:] {
		index, err := parseOBJVertexIndex(field, len(vertices))
		if err != nil {
			return nil, err
		}
		indices = append(indices, index)
	}

	triangles := make([]Triangle, 0, len(indices)-2)
	for index := 1; index < len(indices)-1; index++ {
		triangles = append(triangles, Triangle{
			V0: vertices[indices[0]],
			V1: vertices[indices[index]],
			V2: vertices[indices[index+1]],
		})
	}

	return triangles, nil
}

func parseOBJVertexIndex(field string, vertexCount int) (int, error) {
	indexField, _, _ := strings.Cut(field, "/")
	if indexField == "" {
		return 0, errors.New("face vertex reference is empty")
	}

	index, err := strconv.Atoi(indexField)
	if err != nil {
		return 0, fmt.Errorf("parse face index %q: %w", indexField, err)
	}

	resolvedIndex := index
	if resolvedIndex < 0 {
		resolvedIndex = vertexCount + resolvedIndex + 1
	}
	if resolvedIndex <= 0 || resolvedIndex > vertexCount {
		return 0, fmt.Errorf("face vertex index %d out of range", index)
	}

	return resolvedIndex - 1, nil
}
