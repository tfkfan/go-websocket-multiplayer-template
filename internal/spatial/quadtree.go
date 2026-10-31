package spatial

import "sort"

const (
	DefaultNodeCapacity = 8
	DefaultMaxDepth     = 8
)

type Options struct {
	NodeCapacity int
	MaxDepth     int
}

type Body interface {
	Key() string
	Shape() Shape
	Solid() bool
}

type Quadtree struct {
	bounds   Rect
	capacity int
	maxDepth int
	root     *quadNode
	entries  map[string]Body
	length   int
}

type quadNode struct {
	bounds   Rect
	depth    int
	bodies   []Body
	children [4]*quadNode
}

func NewQuadtree(bounds Rect, options Options) *Quadtree {
	if !bounds.IsValid() || bounds.Width() <= 0 || bounds.Height() <= 0 {
		panic("spatial: invalid quadtree bounds")
	}

	capacity := options.NodeCapacity
	if capacity <= 0 {
		capacity = DefaultNodeCapacity
	}
	maxDepth := options.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}

	return &Quadtree{
		bounds:   bounds,
		capacity: capacity,
		maxDepth: maxDepth,
		root:     &quadNode{bounds: bounds},
		entries:  make(map[string]Body),
	}
}

func (q *Quadtree) Bounds() Rect {
	return q.bounds
}

func (q *Quadtree) Len() int {
	return q.length
}

func (q *Quadtree) Insert(body Body) bool {
	if body == nil {
		return false
	}
	key := body.Key()
	if _, exists := q.entries[key]; exists {
		return false
	}
	shape := body.Shape()
	if shape == nil {
		return false
	}
	if !q.inBounds(shape.Bounds()) {
		return false
	}

	q.insertNode(q.root, body)
	q.entries[key] = body
	q.length++
	return true
}

func (q *Quadtree) Remove(key string) (Body, bool) {
	body, ok := q.entries[key]
	if !ok {
		return nil, false
	}
	if !q.removeNode(q.root, key) {
		return nil, false
	}
	delete(q.entries, key)
	q.length--
	return body, true
}

func (q *Quadtree) Update(body Body) bool {
	if body == nil {
		return false
	}
	key := body.Key()
	if _, ok := q.entries[key]; !ok {
		return false
	}
	shape := body.Shape()
	if shape == nil || !q.inBounds(shape.Bounds()) {
		return false
	}
	if !q.removeNode(q.root, key) {
		return false
	}
	q.insertNode(q.root, body)
	q.entries[key] = body
	return true
}

func (q *Quadtree) Rebuild(bodies []Body) {
	q.Clear()
	for _, body := range bodies {
		q.Insert(body)
	}
}

func (q *Quadtree) Clear() {
	q.root = &quadNode{bounds: q.bounds}
	q.entries = make(map[string]Body)
	q.length = 0
}

func (q *Quadtree) QueryShape(shape Shape) []Body {
	if shape == nil {
		return nil
	}
	queryBounds := shape.Bounds()
	if !queryBounds.IsValid() {
		return nil
	}

	seen := make(map[string]struct{})
	matches := make([]Body, 0, 8)
	q.queryNode(q.root, shape, queryBounds, seen, &matches)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Key() < matches[j].Key()
	})
	return matches
}

func (q *Quadtree) Radius(center Vec2, radius float64) []Body {
	if radius < 0 {
		return nil
	}
	return q.QueryShape(Circle{Center: center, Radius: radius})
}

func (q *Quadtree) Nearest(point Vec2, maxDistance float64, limit int) []Body {
	if limit <= 0 {
		return nil
	}

	candidates := make([]nearestCandidate, 0, limit)
	q.nearestNode(q.root, point, maxDistance, limit, &candidates)

	result := make([]Body, len(candidates))
	for i, candidate := range candidates {
		result[i] = candidate.body
	}
	return result
}

type nearestCandidate struct {
	body     Body
	distance float64
}

func (q *Quadtree) insertNode(node *quadNode, body Body) {
	if node.children[0] != nil {
		if child := childContaining(node, body.Shape().Bounds()); child != nil {
			q.insertNode(child, body)
			return
		}
	}

	node.bodies = append(node.bodies, body)
	if len(node.bodies) > q.capacity && node.depth < q.maxDepth && node.children[0] == nil {
		q.subdivide(node)
	}
}

func (q *Quadtree) subdivide(node *quadNode) {
	quadrants := node.bounds.quadrants()
	for i := range quadrants {
		node.children[i] = &quadNode{bounds: quadrants[i], depth: node.depth + 1}
	}

	bodies := node.bodies
	node.bodies = make([]Body, 0, len(bodies))
	for _, body := range bodies {
		if child := childContaining(node, body.Shape().Bounds()); child != nil {
			q.insertNode(child, body)
		} else {
			node.bodies = append(node.bodies, body)
		}
	}
}

func childContaining(node *quadNode, bounds Rect) *quadNode {
	for _, child := range node.children {
		if child != nil && child.bounds.ContainsRect(bounds) {
			return child
		}
	}
	return nil
}

func (q *Quadtree) removeNode(node *quadNode, key string) bool {
	for i, body := range node.bodies {
		if body.Key() != key {
			continue
		}
		node.bodies = append(node.bodies[:i], node.bodies[i+1:]...)
		return true
	}
	for _, child := range node.children {
		if child != nil && q.removeNode(child, key) {
			return true
		}
	}
	return false
}

func (q *Quadtree) queryNode(node *quadNode, shape Shape, queryBounds Rect, seen map[string]struct{}, matches *[]Body) {
	if !node.bounds.Intersects(queryBounds) {
		return
	}
	for _, body := range node.bodies {
		if !shapesIntersect(body.Shape(), shape) {
			continue
		}
		key := body.Key()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		*matches = append(*matches, body)
	}
	for _, child := range node.children {
		if child != nil {
			q.queryNode(child, shape, queryBounds, seen, matches)
		}
	}
}

func (q *Quadtree) nearestNode(node *quadNode, point Vec2, maxDistance float64, limit int, candidates *[]nearestCandidate) {
	nodeDistance := point.DistanceTo(node.bounds.ClampPoint(point))
	if maxDistance > 0 && nodeDistance > maxDistance {
		return
	}
	if len(*candidates) == limit && nodeDistance > (*candidates)[len(*candidates)-1].distance {
		return
	}

	for _, body := range node.bodies {
		distance := body.Shape().Distance(point)
		if maxDistance > 0 && distance > maxDistance {
			continue
		}
		insertCandidate(candidates, nearestCandidate{body: body, distance: distance}, limit)
	}
	for _, child := range node.children {
		if child != nil {
			q.nearestNode(child, point, maxDistance, limit, candidates)
		}
	}
}

func insertCandidate(candidates *[]nearestCandidate, candidate nearestCandidate, limit int) {
	ordered := *candidates
	if len(ordered) == limit {
		worst := ordered[len(ordered)-1]
		if candidate.distance > worst.distance {
			return
		}
		if candidate.distance == worst.distance && candidate.body.Key() >= worst.body.Key() {
			return
		}
	}

	index := sort.Search(len(ordered), func(i int) bool {
		existing := ordered[i]
		if existing.distance != candidate.distance {
			return existing.distance >= candidate.distance
		}
		return existing.body.Key() >= candidate.body.Key()
	})

	ordered = append(ordered, nearestCandidate{})
	copy(ordered[index+1:], ordered[index:])
	ordered[index] = candidate
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	*candidates = ordered
}

func (q *Quadtree) allBodies() []Body {
	bodies := make([]Body, 0, q.length)
	q.collectBodies(q.root, &bodies)
	return bodies
}

func (q *Quadtree) collectBodies(node *quadNode, bodies *[]Body) {
	*bodies = append(*bodies, node.bodies...)
	for _, child := range node.children {
		if child != nil {
			q.collectBodies(child, bodies)
		}
	}
}

func (q *Quadtree) inBounds(bounds Rect) bool {
	return bounds.IsValid() && q.bounds.ContainsRect(bounds)
}
