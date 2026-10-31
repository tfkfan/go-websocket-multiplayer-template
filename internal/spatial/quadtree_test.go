package spatial

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

type testBody struct {
	key   string
	shape Shape
	solid bool
}

func (b *testBody) Key() string  { return b.key }
func (b *testBody) Shape() Shape { return b.shape }
func (b *testBody) Solid() bool  { return b.solid }

func testWorld() Rect {
	return Rect{Min: Vec2{X: 0, Y: 0}, Max: Vec2{X: 200, Y: 200}}
}

func keysOf(bodies []Body) []string {
	keys := make([]string, len(bodies))
	for i, body := range bodies {
		keys[i] = body.Key()
	}
	return keys
}

func assertKeys(t *testing.T, got []Body, want ...string) {
	t.Helper()
	keys := keysOf(got)
	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %v, want %v", keys, want)
		}
	}
}

func TestNewQuadtreeIsEmpty(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})

	if got := tree.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
	if got := tree.Bounds(); got != testWorld() {
		t.Fatalf("Bounds() = %+v, want %+v", got, testWorld())
	}
	if got := tree.QueryShape(AABB{Rect: testWorld()}); len(got) != 0 {
		t.Fatalf("QueryShape() = %v, want empty", keysOf(got))
	}
	if got := tree.Radius(Vec2{X: 100, Y: 100}, 1000); len(got) != 0 {
		t.Fatalf("Radius() = %v, want empty", keysOf(got))
	}
	if got := tree.Nearest(Vec2{X: 100, Y: 100}, 0, 10); len(got) != 0 {
		t.Fatalf("Nearest() = %v, want empty", keysOf(got))
	}
}

func TestNewQuadtreeFallsBackToDefaults(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{NodeCapacity: -3, MaxDepth: 0})

	if tree.capacity != DefaultNodeCapacity {
		t.Fatalf("capacity = %d, want %d", tree.capacity, DefaultNodeCapacity)
	}
	if tree.maxDepth != DefaultMaxDepth {
		t.Fatalf("maxDepth = %d, want %d", tree.maxDepth, DefaultMaxDepth)
	}

	for i := 0; i < DefaultNodeCapacity*4; i++ {
		body := &testBody{
			key:   fmt.Sprintf("body-%02d", i),
			shape: Circle{Center: Vec2{X: float64(5 + i%10), Y: float64(5 + i/10)}, Radius: 2},
		}
		if !tree.Insert(body) {
			t.Fatalf("Insert(%s) = false", body.key)
		}
	}
	if got := tree.Len(); got != DefaultNodeCapacity*4 {
		t.Fatalf("Len() = %d, want %d", got, DefaultNodeCapacity*4)
	}
}

func TestNewQuadtreeRejectsInvalidBounds(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewQuadtree() did not panic for empty bounds")
		}
	}()
	NewQuadtree(Rect{}, Options{})
}

func TestQuadtreeInsert(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})

	first := &testBody{key: "first", shape: Circle{Center: Vec2{X: 20, Y: 20}, Radius: 5}}
	if !tree.Insert(first) {
		t.Fatal("Insert(first) = false, want true")
	}

	replacement := &testBody{key: "first", shape: Circle{Center: Vec2{X: 150, Y: 150}, Radius: 5}}
	if tree.Insert(replacement) {
		t.Fatal("duplicate Insert() = true, want false")
	}
	got, ok := tree.Remove("first")
	if !ok || got != first {
		t.Fatal("duplicate Insert replaced the original body")
	}

	if tree.Insert(nil) {
		t.Fatal("Insert(nil) = true, want false")
	}

	outside := &testBody{key: "outside", shape: Circle{Center: Vec2{X: 2, Y: 2}, Radius: 5}}
	if tree.Insert(outside) {
		t.Fatal("out-of-bounds Insert() = true, want false")
	}
	if tree.Insert(&testBody{key: "no-shape"}) {
		t.Fatal("shape-less Insert() = true, want false")
	}
}

func TestQuadtreeRemoveAndUpdate(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	body := &testBody{key: "mover", shape: Circle{Center: Vec2{X: 20, Y: 20}, Radius: 5}}
	tree.Insert(body)

	if _, ok := tree.Remove("missing"); ok {
		t.Fatal("Remove(missing) = true, want false")
	}

	query := AABB{Rect: Rect{Min: Vec2{X: 0, Y: 0}, Max: Vec2{X: 40, Y: 40}}}
	if got := tree.QueryShape(query); len(got) != 1 {
		t.Fatalf("QueryShape() before move = %v, want mover", keysOf(got))
	}

	body.shape = Circle{Center: Vec2{X: 160, Y: 160}, Radius: 5}
	if !tree.Update(body) {
		t.Fatal("Update() = false, want true")
	}
	if got := tree.QueryShape(query); len(got) != 0 {
		t.Fatalf("QueryShape() after move = %v, want empty", keysOf(got))
	}
	movedQuery := AABB{Rect: Rect{Min: Vec2{X: 140, Y: 140}, Max: Vec2{X: 180, Y: 180}}}
	if got := tree.QueryShape(movedQuery); len(got) != 1 {
		t.Fatalf("QueryShape() at new position = %v, want mover", keysOf(got))
	}

	outOfBounds := &testBody{key: "mover", shape: Circle{Center: Vec2{X: 2, Y: 2}, Radius: 5}}
	if tree.Update(outOfBounds) {
		t.Fatal("out-of-bounds Update() = true, want false")
	}
	if got := tree.QueryShape(movedQuery); len(got) != 1 {
		t.Fatalf("rejected Update() removed the stored body: %v", keysOf(got))
	}

	if tree.Update(&testBody{key: "missing", shape: Circle{Center: Vec2{X: 50, Y: 50}, Radius: 1}}) {
		t.Fatal("Update(missing) = true, want false")
	}

	if got, ok := tree.Remove("mover"); !ok || got != body {
		t.Fatalf("Remove(mover) = (%v, %v), want the stored body", got, ok)
	}
	if got := tree.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
}

func TestQuadtreeRebuildAndClear(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	old := &testBody{key: "old", shape: Circle{Center: Vec2{X: 20, Y: 20}, Radius: 5}}
	tree.Insert(old)

	first := &testBody{key: "a", shape: Circle{Center: Vec2{X: 60, Y: 60}, Radius: 5}}
	second := &testBody{key: "b", shape: AABB{Rect: Rect{Min: Vec2{X: 100, Y: 100}, Max: Vec2{X: 120, Y: 120}}}}
	duplicate := &testBody{key: "a", shape: Circle{Center: Vec2{X: 170, Y: 170}, Radius: 5}}
	outside := &testBody{key: "outside", shape: Circle{Center: Vec2{X: 1, Y: 1}, Radius: 5}}
	tree.Rebuild([]Body{first, second, duplicate, outside, nil})

	if got := tree.Len(); got != 2 {
		t.Fatalf("Len() after Rebuild = %d, want 2", got)
	}
	if got := tree.QueryShape(AABB{Rect: testWorld()}); len(got) != 2 {
		t.Fatalf("QueryShape() after Rebuild = %v, want a and b", keysOf(got))
	}
	if _, ok := tree.Remove("old"); ok {
		t.Fatal("Rebuild kept the previous body")
	}

	tree.Clear()
	if got := tree.Len(); got != 0 {
		t.Fatalf("Len() after Clear = %d, want 0", got)
	}
	if got := tree.QueryShape(AABB{Rect: testWorld()}); len(got) != 0 {
		t.Fatalf("QueryShape() after Clear = %v, want empty", keysOf(got))
	}
	if !tree.Insert(first) {
		t.Fatal("Insert() after Clear = false, want true")
	}
}

func TestQueryShapeUsesExactGeometry(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	corner := &testBody{key: "corner", shape: Circle{Center: Vec2{X: 10, Y: 10}, Radius: 4}}
	tree.Insert(corner)

	query := AABB{Rect: Rect{Min: Vec2{X: 13.9, Y: 13.9}, Max: Vec2{X: 16, Y: 16}}}
	if got := tree.QueryShape(query); len(got) != 0 {
		t.Fatalf("QueryShape() = %v, want no exact intersection", keysOf(got))
	}

	hit := &testBody{key: "hit", shape: Circle{Center: Vec2{X: 14.5, Y: 14.5}, Radius: 2}}
	tree.Insert(hit)
	assertKeys(t, tree.QueryShape(query), "hit")
}

func TestQueryShapeOrdering(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	for _, key := range []string{"b", "c", "a"} {
		body := &testBody{key: key, shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}}
		if !tree.Insert(body) {
			t.Fatalf("Insert(%s) = false", key)
		}
	}
	assertKeys(t, tree.QueryShape(AABB{Rect: testWorld()}), "a", "b", "c")
}

func TestRadiusQuery(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	inside := &testBody{key: "inside", shape: Circle{Center: Vec2{X: 50, Y: 50}, Radius: 2}}
	edge := &testBody{key: "edge", shape: AABB{Rect: Rect{Min: Vec2{X: 70, Y: 45}, Max: Vec2{X: 75, Y: 55}}}}
	outside := &testBody{key: "outside", shape: Circle{Center: Vec2{X: 150, Y: 150}, Radius: 2}}
	for _, body := range []*testBody{inside, edge, outside} {
		tree.Insert(body)
	}

	assertKeys(t, tree.Radius(Vec2{X: 50, Y: 50}, 30), "edge", "inside")
	if got := tree.Radius(Vec2{X: 50, Y: 50}, -1); got != nil {
		t.Fatalf("Radius(-1) = %v, want nil", keysOf(got))
	}
}

func TestNearestQuery(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	tree.Insert(&testBody{key: "far", shape: Circle{Center: Vec2{X: 150, Y: 100}, Radius: 1}})
	tree.Insert(&testBody{key: "near-b", shape: Circle{Center: Vec2{X: 40, Y: 100}, Radius: 1}})
	tree.Insert(&testBody{key: "near-a", shape: Circle{Center: Vec2{X: 60, Y: 100}, Radius: 1}})

	assertKeys(t, tree.Nearest(Vec2{X: 50, Y: 100}, 0, 0))
	assertKeys(t, tree.Nearest(Vec2{X: 50, Y: 100}, 0, 10), "near-a", "near-b", "far")
	assertKeys(t, tree.Nearest(Vec2{X: 50, Y: 100}, 0, 2), "near-a", "near-b")

	tieTree := NewQuadtree(testWorld(), Options{})
	tieTree.Insert(&testBody{key: "z", shape: Circle{Center: Vec2{X: 20, Y: 20}, Radius: 1}})
	tieTree.Insert(&testBody{key: "a", shape: Circle{Center: Vec2{X: 20, Y: 60}, Radius: 1}})
	assertKeys(t, tieTree.Nearest(Vec2{X: 20, Y: 40}, 0, 2), "a", "z")

	assertKeys(t, tree.Nearest(Vec2{X: 20, Y: 100}, 19, 10), "near-b")
	if got := tree.Nearest(Vec2{X: 20, Y: 100}, 18.9, 10); len(got) != 0 {
		t.Fatalf("Nearest() beyond the limit = %v, want empty", keysOf(got))
	}

	insideTree := NewQuadtree(testWorld(), Options{})
	insideTree.Insert(&testBody{key: "box", shape: AABB{Rect: Rect{Min: Vec2{X: 40, Y: 40}, Max: Vec2{X: 60, Y: 60}}}})
	insideTree.Insert(&testBody{key: "dot", shape: Circle{Center: Vec2{X: 50, Y: 50}, Radius: 2}})
	assertKeys(t, insideTree.Nearest(Vec2{X: 50, Y: 50}, 0, 2), "box", "dot")

	assertKeys(t, tree.Nearest(Vec2{X: 180, Y: 150}, 60, 1), "far")
}

func TestQuadtreeMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	const bodyCount = 200

	bodies := make([]*testBody, 0, bodyCount)
	for i := 0; i < bodyCount; i++ {
		key := fmt.Sprintf("body-%03d", i)
		if i%2 == 0 {
			radius := 1 + rng.Float64()*9
			center := Vec2{
				X: 10 + rng.Float64()*(testWorld().Width()-20),
				Y: 10 + rng.Float64()*(testWorld().Height()-20),
			}
			bodies = append(bodies, &testBody{key: key, shape: Circle{Center: center, Radius: radius}})
			continue
		}
		width := 2 + rng.Float64()*20
		height := 2 + rng.Float64()*20
		minX := rng.Float64() * (testWorld().Width() - width)
		minY := rng.Float64() * (testWorld().Height() - height)
		bodies = append(bodies, &testBody{
			key:   key,
			shape: AABB{Rect: Rect{Min: Vec2{X: minX, Y: minY}, Max: Vec2{X: minX + width, Y: minY + height}}},
		})
	}

	tree := NewQuadtree(testWorld(), Options{})
	reversed := make([]Body, 0, len(bodies))
	for i := len(bodies) - 1; i >= 0; i-- {
		reversed = append(reversed, bodies[i])
	}
	tree.Rebuild(reversed)

	for queryIndex := 0; queryIndex < 20; queryIndex++ {
		center := Vec2{
			X: rng.Float64() * testWorld().Width(),
			Y: rng.Float64() * testWorld().Height(),
		}
		radius := 5 + rng.Float64()*50
		query := Circle{Center: center, Radius: radius}

		got := keysOf(tree.QueryShape(query))
		want := bruteQuery(bodies, query)
		if len(got) != len(want) {
			t.Fatalf("QueryShape(%+v) = %v, want %v", query, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("QueryShape(%+v) = %v, want %v", query, got, want)
			}
		}
	}

	for queryIndex := 0; queryIndex < 20; queryIndex++ {
		point := Vec2{
			X: rng.Float64() * testWorld().Width(),
			Y: rng.Float64() * testWorld().Height(),
		}
		limit := 1 + rng.Intn(10)
		maxDistance := 0.0
		if queryIndex%2 == 0 {
			maxDistance = 20 + rng.Float64()*80
		}

		got := keysOf(tree.Nearest(point, maxDistance, limit))
		want := bruteNearest(bodies, point, maxDistance, limit)
		if len(got) != len(want) {
			t.Fatalf("Nearest(%+v, %v, %d) = %v, want %v", point, maxDistance, limit, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Nearest(%+v, %v, %d) = %v, want %v", point, maxDistance, limit, got, want)
			}
		}
	}
}

func bruteQuery(bodies []*testBody, query Shape) []string {
	keys := make([]string, 0)
	for _, body := range bodies {
		if shapesIntersect(body.Shape(), query) {
			keys = append(keys, body.Key())
		}
	}
	sort.Strings(keys)
	return keys
}

func bruteNearest(bodies []*testBody, point Vec2, maxDistance float64, limit int) []string {
	type candidate struct {
		key      string
		distance float64
	}
	candidates := make([]candidate, 0, len(bodies))
	for _, body := range bodies {
		distance := body.Shape().Distance(point)
		if maxDistance > 0 && distance > maxDistance {
			continue
		}
		candidates = append(candidates, candidate{key: body.Key(), distance: distance})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].key < candidates[j].key
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	keys := make([]string, len(candidates))
	for i, candidate := range candidates {
		keys[i] = candidate.key
	}
	return keys
}

func BenchmarkQuadtreeRebuildAndNearest(b *testing.B) {
	const bodyCount = 1000

	world := testWorld()
	rng := rand.New(rand.NewSource(7))
	bodies := make([]Body, 0, bodyCount)
	for i := 0; i < bodyCount; i++ {
		bodies = append(bodies, &testBody{
			key: fmt.Sprintf("bench-%04d", i),
			shape: Circle{
				Center: Vec2{
					X: 10 + rng.Float64()*(world.Width()-20),
					Y: 10 + rng.Float64()*(world.Height()-20),
				},
				Radius: 5,
			},
		})
	}

	tree := NewQuadtree(world, Options{})
	point := world.Center()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Rebuild(bodies)
		if got := tree.Nearest(point, 0, 8); len(got) != 8 {
			b.Fatalf("Nearest() returned %d bodies, want 8", len(got))
		}
	}
}
