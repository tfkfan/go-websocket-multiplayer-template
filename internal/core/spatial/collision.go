package spatial

import "sort"

type Contact struct {
	A      Body
	B      Body
	Normal Vec2
	Depth  float64
}

func Detect(a, b Body) (Contact, bool) {
	if a == nil || b == nil || a.Key() == b.Key() {
		return Contact{}, false
	}

	normal, depth, ok := shapePenetration(a.Shape(), b.Shape())
	if !ok || depth <= epsilon {
		return Contact{}, false
	}

	if a.Key() <= b.Key() {
		return Contact{A: a, B: b, Normal: normal, Depth: depth}, true
	}
	return Contact{A: b, B: a, Normal: normal.Scale(-1), Depth: depth}, true
}

func DetectCollisions(index *Quadtree, onContact func(Contact)) []Contact {
	if index == nil {
		return nil
	}

	pairs := index.solidPairs()
	contacts := make([]Contact, 0, len(pairs))
	for _, pair := range pairs {
		contact, ok := Detect(pair.a, pair.b)
		if !ok || contact.Depth <= epsilon {
			continue
		}
		contacts = append(contacts, contact)
		if onContact != nil {
			onContact(contact)
		}
	}
	return contacts
}

func DetectBodyCollisions(index *Quadtree, body Body, onContact func(Contact)) []Contact {
	if index == nil || body == nil || body.Shape() == nil || !body.Solid() {
		return nil
	}

	others := index.QueryShape(body.Shape())
	contacts := make([]Contact, 0, len(others))
	for _, other := range others {
		if !other.Solid() || other.Key() == body.Key() {
			continue
		}
		contact, ok := Detect(body, other)
		if !ok || contact.Depth <= epsilon {
			continue
		}
		contacts = append(contacts, contact)
		if onContact != nil {
			onContact(contact)
		}
	}
	return contacts
}

type bodyPair struct {
	a Body
	b Body
}

func (q *Quadtree) solidPairs() []bodyPair {
	seen := make(map[[2]string]struct{})
	pairs := make([]bodyPair, 0)

	for _, body := range q.allBodies() {
		if !body.Solid() {
			continue
		}
		for _, other := range q.QueryShape(body.Shape()) {
			if !other.Solid() || other.Key() == body.Key() {
				continue
			}

			firstKey, secondKey := body.Key(), other.Key()
			first, second := body, other
			if firstKey > secondKey {
				firstKey, secondKey = secondKey, firstKey
				first, second = second, first
			}

			key := [2]string{firstKey, secondKey}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			pairs = append(pairs, bodyPair{a: first, b: second})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].a.Key() != pairs[j].a.Key() {
			return pairs[i].a.Key() < pairs[j].a.Key()
		}
		return pairs[i].b.Key() < pairs[j].b.Key()
	})
	return pairs
}

func shapePenetration(a, b Shape) (Vec2, float64, bool) {
	switch first := a.(type) {
	case Circle:
		switch second := b.(type) {
		case Circle:
			delta := first.Center.Sub(second.Center)
			distance := delta.Length()
			if distance <= epsilon {
				return Vec2{X: 1}, first.Radius + second.Radius, first.Radius+second.Radius > epsilon
			}
			depth := first.Radius + second.Radius - distance
			return delta.Scale(1 / distance), depth, depth > 0
		case AABB:
			result := circleBoxPenetration(first, second)
			return result.normal, result.depth, result.depth > 0
		}
	case AABB:
		switch second := b.(type) {
		case Circle:
			result := circleBoxPenetration(second, first)
			return result.normal.Scale(-1), result.depth, result.depth > 0
		case AABB:
			return boxBoxPenetration(first, second)
		}
	}
	return Vec2{}, 0, false
}

func boxBoxPenetration(a, b AABB) (Vec2, float64, bool) {
	overlapX, overlapY := rectPenetration(a.Rect, b.Rect)
	if overlapX <= 0 || overlapY <= 0 {
		return Vec2{}, 0, false
	}

	delta := a.CenterPoint().Sub(b.CenterPoint())
	if overlapX < overlapY {
		normal := Vec2{X: 1}
		if delta.X < 0 {
			normal = Vec2{X: -1}
		}
		return normal, overlapX, true
	}

	normal := Vec2{Y: 1}
	if delta.Y < 0 {
		normal = Vec2{Y: -1}
	}
	return normal, overlapY, true
}
