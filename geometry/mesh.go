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
//
//nolint:errname // The value intentionally groups both errors and non-fatal warnings.
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

// MeshFromBox creates a 12-triangle mesh for an axis-aligned box.
// Winding is CCW when viewed from inside the box (normals point inward).
func MeshFromBox(bmin, bmax Vec3) *Mesh {
	// 8 vertices of the box.
	v := [8]Vec3{
		{bmin.X, bmin.Y, bmin.Z}, // 0: ---
		{bmax.X, bmin.Y, bmin.Z}, // 1: +--
		{bmax.X, bmax.Y, bmin.Z}, // 2: ++-
		{bmin.X, bmax.Y, bmin.Z}, // 3: -+-
		{bmin.X, bmin.Y, bmax.Z}, // 4: --+
		{bmax.X, bmin.Y, bmax.Z}, // 5: +-+
		{bmax.X, bmax.Y, bmax.Z}, // 6: +++
		{bmin.X, bmax.Y, bmax.Z}, // 7: -++
	}

	// Each face is two triangles. Winding is CCW when viewed from inside,
	// so the computed normal (V1-V0)×(V2-V0) points inward.
	triangles := []Triangle{
		// -X face (x = bmin.X): inward normal = +X
		{V0: v[0], V1: v[3], V2: v[4]},
		{V0: v[3], V1: v[7], V2: v[4]},
		// +X face (x = bmax.X): inward normal = -X
		{V0: v[1], V1: v[5], V2: v[2]},
		{V0: v[2], V1: v[5], V2: v[6]},
		// -Y face (y = bmin.Y): inward normal = +Y
		{V0: v[0], V1: v[4], V2: v[1]},
		{V0: v[1], V1: v[4], V2: v[5]},
		// +Y face (y = bmax.Y): inward normal = -Y
		{V0: v[3], V1: v[2], V2: v[7]},
		{V0: v[2], V1: v[6], V2: v[7]},
		// -Z face (z = bmin.Z): inward normal = +Z
		{V0: v[0], V1: v[1], V2: v[3]},
		{V0: v[1], V1: v[2], V2: v[3]},
		// +Z face (z = bmax.Z): inward normal = -Z
		{V0: v[4], V1: v[7], V2: v[5]},
		{V0: v[5], V1: v[7], V2: v[6]},
	}

	return &Mesh{Triangles: triangles}
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
		if !isFiniteMeshVertex(tri.V0) || !isFiniteMeshVertex(tri.V1) || !isFiniteMeshVertex(tri.V2) {
			issues.Problems = append(issues.Problems, fmt.Sprintf("triangle[%d] contains a non-finite vertex", index))
			continue
		}

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

func isFiniteMeshVertex(v Vec3) bool {
	return !math.IsNaN(v.X) && !math.IsInf(v.X, 0) &&
		!math.IsNaN(v.Y) && !math.IsInf(v.Y, 0) &&
		!math.IsNaN(v.Z) && !math.IsInf(v.Z, 0)
}

// LoadOBJ loads a minimal triangle mesh from an OBJ file. Only vertex and face
// records are used; common metadata records are ignored.
func LoadOBJ(path string) (*Mesh, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open OBJ %q: %w", path, err)
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

		parseErr := appendOBJRecord(fields, &vertices, &triangles)
		if parseErr != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, parseErr)
		}
	}

	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("scan OBJ %q: %w", path, err)
	}

	mesh := &Mesh{Triangles: triangles}

	validationErr := mesh.Validate()
	if validationErr != nil {
		var issues *MeshValidationIssues
		if errors.As(validationErr, &issues) && !issues.HasProblems() {
			return mesh, nil
		}

		return nil, validationErr
	}

	return mesh, nil
}

func appendOBJRecord(fields []string, vertices *[]Vec3, triangles *[]Triangle) error {
	switch fields[0] {
	case "v":
		vertex, err := parseOBJVertex(fields)
		if err != nil {
			return err
		}

		*vertices = append(*vertices, vertex)
	case "f":
		faceTriangles, err := parseOBJFace(fields, *vertices)
		if err != nil {
			return err
		}

		*triangles = append(*triangles, faceTriangles...)
	case "vt", "vn", "o", "g", "s", "usemtl", "mtllib":
		return nil
	default:
		return fmt.Errorf("unsupported OBJ record %q", fields[0])
	}

	return nil
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

type meshEdgeUse struct {
	count       int
	orientation int
	first       int
	second      int
}

// EnclosedVolume returns the signed-volume estimate for a single, consistently
// oriented, topologically watertight mesh. Open, non-manifold, disconnected, or
// inconsistently wound meshes do not have an unambiguous room volume and return
// false. As with the standard tetrahedral-volume formula, the surface is assumed
// not to self-intersect.
func (m *Mesh) EnclosedVolume() (float64, bool) {
	if m == nil || len(m.Triangles) == 0 {
		return 0, false
	}

	edges := make(map[meshEdgeKey]meshEdgeUse, len(m.Triangles)*3)
	origin := m.Triangles[0].V0
	signedSixVolume := 0.0

	for triangleIndex, triangle := range m.Triangles {
		if !isFiniteMeshVertex(triangle.V0) ||
			!isFiniteMeshVertex(triangle.V1) ||
			!isFiniteMeshVertex(triangle.V2) ||
			triangle.Area() <= meshDegenerateAreaEpsilon {
			return 0, false
		}

		signedSixVolume += triangle.V0.Sub(origin).Dot(
			triangle.V1.Sub(origin).Cross(triangle.V2.Sub(origin)),
		)
		addMeshEdgeUse(edges, triangle.V0, triangle.V1, triangleIndex)
		addMeshEdgeUse(edges, triangle.V1, triangle.V2, triangleIndex)
		addMeshEdgeUse(edges, triangle.V2, triangle.V0, triangleIndex)
	}

	adjacency := make([][]int, len(m.Triangles))

	for _, edge := range edges {
		if edge.count != 2 || edge.orientation != 0 {
			return 0, false
		}

		adjacency[edge.first] = append(adjacency[edge.first], edge.second)
		adjacency[edge.second] = append(adjacency[edge.second], edge.first)
	}

	if !meshTrianglesConnected(adjacency) {
		return 0, false
	}

	volume := math.Abs(signedSixVolume) / 6
	if math.IsNaN(volume) || math.IsInf(volume, 0) || volume <= 0 {
		return 0, false
	}

	return volume, true
}

func addMeshEdgeUse(edges map[meshEdgeKey]meshEdgeUse, a, b Vec3, triangleIndex int) {
	key := newMeshEdgeKey(a, b)

	use := edges[key]
	switch use.count {
	case 0:
		use.first = triangleIndex
	case 1:
		use.second = triangleIndex
	}

	use.count++
	if newMeshVertexKey(a) == key.A {
		use.orientation++
	} else {
		use.orientation--
	}

	edges[key] = use
}

func meshTrianglesConnected(adjacency [][]int) bool {
	visited := make([]bool, len(adjacency))
	stack := []int{0}
	visited[0] = true
	visitedCount := 1

	for len(stack) > 0 {
		last := len(stack) - 1
		triangleIndex := stack[last]
		stack = stack[:last]

		for _, neighbor := range adjacency[triangleIndex] {
			if visited[neighbor] {
				continue
			}

			visited[neighbor] = true
			visitedCount++

			stack = append(stack, neighbor)
		}
	}

	return visitedCount == len(adjacency)
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
