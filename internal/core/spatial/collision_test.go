package spatial

import (
	"math"
	"testing"
)

func TestContactCircleCircle(t *testing.T) {
	a := &testBody{key: "a", shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}}
	b := &testBody{key: "b", shape: Circle{Center: Vec2{X: 108, Y: 100}, Radius: 5}}

	contact, ok := Detect(a, b)
	if !ok {
		t.Fatal("Detect() = false, want a contact")
	}
	if contact.A != a || contact.B != b {
		t.Fatal("Detect() did not preserve key ordering")
	}
	assertNear(t, contact.Depth, 2, 1e-9)
	assertVecNear(t, contact.Normal, Vec2{X: -1}, 1e-9)

	touching := &testBody{key: "b", shape: Circle{Center: Vec2{X: 110, Y: 100}, Radius: 5}}
	if _, ok := Detect(a, touching); ok {
		t.Fatal("touching circles produced a contact")
	}

	separated := &testBody{key: "b", shape: Circle{Center: Vec2{X: 112, Y: 100}, Radius: 5}}
	if _, ok := Detect(a, separated); ok {
		t.Fatal("separated circles produced a contact")
	}

	coincident := &testBody{key: "b", shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}}
	contact, ok = Detect(a, coincident)
	if !ok {
		t.Fatal("coincident circles produced no contact")
	}
	assertNear(t, contact.Depth, 10, 1e-9)
	assertVecNear(t, contact.Normal, Vec2{X: 1}, 1e-9)
}

func TestContactOrdersByKey(t *testing.T) {
	left := &testBody{key: "z", shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}}
	right := &testBody{key: "b", shape: Circle{Center: Vec2{X: 106, Y: 100}, Radius: 5}}

	contact, ok := Detect(left, right)
	if !ok {
		t.Fatal("Detect() = false, want a contact")
	}
	if contact.A.Key() != "b" || contact.B.Key() != "z" {
		t.Fatalf("Detect() ordered keys as %s, %s", contact.A.Key(), contact.B.Key())
	}
	assertVecNear(t, contact.Normal, Vec2{X: 1}, 1e-9)
	assertNear(t, contact.Depth, 4, 1e-9)
}

func TestContactCircleBox(t *testing.T) {
	box := &testBody{key: "box", shape: AABB{Rect: Rect{Min: Vec2{X: 0, Y: 0}, Max: Vec2{X: 10, Y: 10}}}}

	face := &testBody{key: "aaa", shape: Circle{Center: Vec2{X: 12, Y: 5}, Radius: 5}}
	contact, ok := Detect(face, box)
	if !ok {
		t.Fatal("face overlap produced no contact")
	}
	assertNear(t, contact.Depth, 3, 1e-9)
	assertVecNear(t, contact.Normal, Vec2{X: 1}, 1e-9)

	inside := &testBody{key: "aaa", shape: Circle{Center: Vec2{X: 8, Y: 5}, Radius: 5}}
	contact, ok = Detect(inside, box)
	if !ok {
		t.Fatal("inside overlap produced no contact")
	}
	assertNear(t, contact.Depth, 7, 1e-9)
	assertVecNear(t, contact.Normal, Vec2{X: 1}, 1e-9)

	corner := &testBody{key: "aaa", shape: Circle{Center: Vec2{X: 12, Y: 12}, Radius: 5}}
	contact, ok = Detect(corner, box)
	if !ok {
		t.Fatal("corner overlap produced no contact")
	}
	assertNear(t, contact.Depth, 5-math.Sqrt(8), 1e-9)
	assertVecNear(t, contact.Normal, Vec2{X: math.Sqrt2 / 2, Y: math.Sqrt2 / 2}, 1e-9)

	right := &testBody{key: "zzz", shape: Circle{Center: Vec2{X: 12, Y: 5}, Radius: 5}}
	contact, ok = Detect(right, box)
	if !ok {
		t.Fatal("reversed overlap produced no contact")
	}
	if contact.A != box || contact.B != right {
		t.Fatal("reversed pair was not key-ordered")
	}
	assertVecNear(t, contact.Normal, Vec2{X: -1}, 1e-9)
}

func TestContactBoxBox(t *testing.T) {
	a := &testBody{key: "a", shape: AABB{Rect: Rect{Min: Vec2{X: 0, Y: 0}, Max: Vec2{X: 10, Y: 10}}}}

	xOverlap := &testBody{key: "b", shape: AABB{Rect: Rect{Min: Vec2{X: 8, Y: 0}, Max: Vec2{X: 18, Y: 10}}}}
	contact, ok := Detect(a, xOverlap)
	if !ok {
		t.Fatal("x overlap produced no contact")
	}
	assertNear(t, contact.Depth, 2, 1e-9)
	assertVecNear(t, contact.Normal, Vec2{X: -1}, 1e-9)

	yOverlap := &testBody{key: "b", shape: AABB{Rect: Rect{Min: Vec2{X: 0, Y: 8}, Max: Vec2{X: 10, Y: 18}}}}
	contact, ok = Detect(a, yOverlap)
	if !ok {
		t.Fatal("y overlap produced no contact")
	}
	assertNear(t, contact.Depth, 2, 1e-9)
	assertVecNear(t, contact.Normal, Vec2{Y: -1}, 1e-9)

	touching := &testBody{key: "b", shape: AABB{Rect: Rect{Min: Vec2{X: 10, Y: 0}, Max: Vec2{X: 20, Y: 10}}}}
	if _, ok := Detect(a, touching); ok {
		t.Fatal("touching boxes produced a contact")
	}
}

func TestDetectCollisionsReportsContactsInKeyOrder(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	first := &testBody{key: "a", solid: true, shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}}
	second := &testBody{key: "b", solid: true, shape: Circle{Center: Vec2{X: 104, Y: 100}, Radius: 5}}
	third := &testBody{key: "c", solid: true, shape: AABB{Rect: Rect{Min: Vec2{X: 98, Y: 98}, Max: Vec2{X: 102, Y: 102}}}}
	for _, body := range []*testBody{second, third, first} {
		tree.Insert(body)
	}

	firstPositions := map[string]Vec2{
		"a": first.Shape().CenterPoint(),
		"b": second.Shape().CenterPoint(),
		"c": third.Shape().CenterPoint(),
	}

	var callbackKeys []string
	contacts := DetectCollisions(tree, func(contact Contact) {
		callbackKeys = append(callbackKeys, contact.A.Key()+"-"+contact.B.Key())
	})

	if len(contacts) != 3 {
		t.Fatalf("DetectCollisions() returned %d contacts, want 3", len(contacts))
	}
	want := []string{"a-b", "a-c", "b-c"}
	for i, pair := range want {
		if callbackKeys[i] != pair {
			t.Fatalf("callback order = %v, want %v", callbackKeys, want)
		}
		if contacts[i].A.Key()+"-"+contacts[i].B.Key() != pair {
			t.Fatalf("contact order = %v, want %v", contactKeys(contacts), want)
		}
	}

	for _, body := range []*testBody{first, second, third} {
		if got := body.Shape().CenterPoint(); got != firstPositions[body.Key()] {
			t.Fatalf("body %s moved from %+v to %+v", body.Key(), firstPositions[body.Key()], got)
		}
	}
}

func TestDetectCollisionsIgnoresTriggers(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	tree.Insert(&testBody{key: "solid", solid: true, shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}})
	tree.Insert(&testBody{key: "trigger", solid: false, shape: Circle{Center: Vec2{X: 104, Y: 100}, Radius: 5}})
	tree.Insert(&testBody{key: "other-trigger", solid: false, shape: Circle{Center: Vec2{X: 101, Y: 100}, Radius: 5}})

	called := false
	contacts := DetectCollisions(tree, func(Contact) { called = true })
	if len(contacts) != 0 || called {
		t.Fatalf("DetectCollisions() reported %d contacts for non-solid bodies", len(contacts))
	}
}

func TestDetectCollisionsHandlesNilInputs(t *testing.T) {
	if got := DetectCollisions(nil, func(Contact) { t.Fatal("callback ran for a nil index") }); got != nil {
		t.Fatalf("DetectCollisions(nil) = %v, want nil", got)
	}

	tree := NewQuadtree(testWorld(), Options{})
	tree.Insert(&testBody{key: "a", solid: true, shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}})
	tree.Insert(&testBody{key: "b", solid: true, shape: Circle{Center: Vec2{X: 104, Y: 100}, Radius: 5}})

	if got := DetectCollisions(tree, nil); len(got) != 1 {
		t.Fatalf("DetectCollisions(nil callback) returned %d contacts, want 1", len(got))
	}
}

func TestDetectCollisionsSkipsSeparatedBodies(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	tree.Insert(&testBody{key: "a", solid: true, shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}})
	tree.Insert(&testBody{key: "b", solid: true, shape: Circle{Center: Vec2{X: 120, Y: 100}, Radius: 5}})

	called := false
	if got := DetectCollisions(tree, func(Contact) { called = true }); len(got) != 0 || called {
		t.Fatalf("DetectCollisions() reported %d contacts for separated bodies", len(got))
	}
}

func contactKeys(contacts []Contact) []string {
	keys := make([]string, len(contacts))
	for i, contact := range contacts {
		keys[i] = contact.A.Key() + "-" + contact.B.Key()
	}
	return keys
}

func assertNear(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("value = %.12f, want %.12f (tolerance %g)", got, want, tolerance)
	}
}

func assertVecNear(t *testing.T, got, want Vec2, tolerance float64) {
	t.Helper()
	if math.Abs(got.X-want.X) > tolerance || math.Abs(got.Y-want.Y) > tolerance {
		t.Fatalf("vec = %+v, want %+v (tolerance %g)", got, want, tolerance)
	}
}

func TestDetectBodyCollisionsReportsMoverContacts(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	mover := &testBody{key: "b", solid: true, shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}}
	left := &testBody{key: "a", solid: true, shape: Circle{Center: Vec2{X: 97, Y: 100}, Radius: 5}}
	right := &testBody{key: "c", solid: true, shape: Circle{Center: Vec2{X: 103, Y: 100}, Radius: 5}}
	far := &testBody{key: "z", solid: true, shape: Circle{Center: Vec2{X: 150, Y: 150}, Radius: 5}}
	for _, body := range []*testBody{right, far, mover, left} {
		tree.Insert(body)
	}

	start := mover.Shape().CenterPoint()
	var order []string
	contacts := DetectBodyCollisions(tree, mover, func(contact Contact) {
		other := contact.A
		if other.Key() == mover.Key() {
			other = contact.B
		}
		order = append(order, other.Key())
	})
	if len(contacts) != 2 {
		t.Fatalf("DetectBodyCollisions() returned %d contacts, want 2", len(contacts))
	}
	if len(order) != 2 || order[0] != "a" || order[1] != "c" {
		t.Fatalf("callback order = %v, want [a c]", order)
	}
	if got := mover.Shape().CenterPoint(); got != start {
		t.Fatalf("mover moved from %+v to %+v during detection", start, got)
	}
}

func TestDetectBodyCollisionsIgnoresNonSolid(t *testing.T) {
	tree := NewQuadtree(testWorld(), Options{})
	mover := &testBody{key: "mover", solid: true, shape: Circle{Center: Vec2{X: 100, Y: 100}, Radius: 5}}
	trigger := &testBody{key: "trigger", solid: false, shape: Circle{Center: Vec2{X: 101, Y: 100}, Radius: 5}}
	tree.Insert(mover)
	tree.Insert(trigger)

	called := false
	if got := DetectBodyCollisions(tree, mover, func(Contact) { called = true }); len(got) != 0 || called {
		t.Fatalf("DetectBodyCollisions() reported %d trigger contacts", len(got))
	}
}

func TestDetectBodyCollisionsHandlesNilInputs(t *testing.T) {
	if got := DetectBodyCollisions(nil, &testBody{key: "a", solid: true, shape: Circle{Radius: 1}}, func(Contact) {
		t.Fatal("callback ran for a nil index")
	}); got != nil {
		t.Fatalf("DetectBodyCollisions(nil) = %v, want nil", got)
	}

	tree := NewQuadtree(testWorld(), Options{})
	if got := DetectBodyCollisions(tree, nil, func(Contact) {
		t.Fatal("callback ran for a nil body")
	}); got != nil {
		t.Fatalf("DetectBodyCollisions(nil body) = %v, want nil", got)
	}

	if got := DetectBodyCollisions(tree, &testBody{key: "no-shape", solid: true}, func(Contact) {
		t.Fatal("callback ran for a body without a shape")
	}); got != nil {
		t.Fatalf("DetectBodyCollisions(no shape) = %v, want nil", got)
	}
}
