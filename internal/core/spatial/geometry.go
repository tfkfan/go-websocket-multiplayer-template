package spatial

import (
	"math"
)

const epsilon = 1e-9

type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{X: v.X + other.X, Y: v.Y + other.Y}
}

func (v Vec2) Sub(other Vec2) Vec2 {
	return Vec2{X: v.X - other.X, Y: v.Y - other.Y}
}

func (v Vec2) Scale(factor float64) Vec2 {
	return Vec2{X: v.X * factor, Y: v.Y * factor}
}

func (v Vec2) Dot(other Vec2) float64 {
	return v.X*other.X + v.Y*other.Y
}

func (v Vec2) LengthSquared() float64 {
	return v.Dot(v)
}

func (v Vec2) Length() float64 {
	return math.Sqrt(v.LengthSquared())
}

func (v Vec2) DistanceTo(other Vec2) float64 {
	return v.Sub(other).Length()
}

func (v Vec2) Normalized() Vec2 {
	length := v.Length()
	if length <= epsilon {
		return Vec2{}
	}
	return v.Scale(1 / length)
}

type Rect struct {
	Min Vec2
	Max Vec2
}

func (r Rect) Width() float64 {
	return r.Max.X - r.Min.X
}

func (r Rect) Height() float64 {
	return r.Max.Y - r.Min.Y
}

func (r Rect) Center() Vec2 {
	return Vec2{X: (r.Min.X + r.Max.X) / 2, Y: (r.Min.Y + r.Max.Y) / 2}
}

func (r Rect) IsValid() bool {
	return isFinite(r.Min.X) && isFinite(r.Min.Y) &&
		isFinite(r.Max.X) && isFinite(r.Max.Y) &&
		r.Min.X <= r.Max.X && r.Min.Y <= r.Max.Y
}

func (r Rect) ContainsPoint(point Vec2) bool {
	return point.X >= r.Min.X && point.X <= r.Max.X &&
		point.Y >= r.Min.Y && point.Y <= r.Max.Y
}

func (r Rect) Intersects(other Rect) bool {
	return r.Min.X <= other.Max.X && other.Min.X <= r.Max.X &&
		r.Min.Y <= other.Max.Y && other.Min.Y <= r.Max.Y
}

func (r Rect) ContainsRect(other Rect) bool {
	return other.Min.X >= r.Min.X && other.Min.Y >= r.Min.Y &&
		other.Max.X <= r.Max.X && other.Max.Y <= r.Max.Y
}

func (r Rect) ClampPoint(point Vec2) Vec2 {
	return Vec2{
		X: math.Min(math.Max(point.X, r.Min.X), r.Max.X),
		Y: math.Min(math.Max(point.Y, r.Min.Y), r.Max.Y),
	}
}

func (r Rect) Translate(delta Vec2) Rect {
	return Rect{Min: r.Min.Add(delta), Max: r.Max.Add(delta)}
}

func (r Rect) quadrants() [4]Rect {
	center := r.Center()
	return [4]Rect{
		{Min: r.Min, Max: center},
		{Min: Vec2{X: center.X, Y: r.Min.Y}, Max: Vec2{X: r.Max.X, Y: center.Y}},
		{Min: Vec2{X: r.Min.X, Y: center.Y}, Max: Vec2{X: center.X, Y: r.Max.Y}},
		{Min: center, Max: r.Max},
	}
}

type Shape interface {
	Bounds() Rect
	CenterPoint() Vec2
	ContainsPoint(point Vec2) bool
	Distance(point Vec2) float64
}

type Circle struct {
	Center Vec2
	Radius float64
}

func (c Circle) Bounds() Rect {
	return Rect{
		Min: Vec2{X: c.Center.X - c.Radius, Y: c.Center.Y - c.Radius},
		Max: Vec2{X: c.Center.X + c.Radius, Y: c.Center.Y + c.Radius},
	}
}

func (c Circle) CenterPoint() Vec2 {
	return c.Center
}

func (c Circle) ContainsPoint(point Vec2) bool {
	return c.Center.DistanceTo(point) <= c.Radius
}

func (c Circle) Distance(point Vec2) float64 {
	return math.Max(0, c.Center.DistanceTo(point)-c.Radius)
}

type AABB struct {
	Rect Rect
}

func (b AABB) Bounds() Rect {
	return b.Rect
}

func (b AABB) CenterPoint() Vec2 {
	return b.Rect.Center()
}

func (b AABB) ContainsPoint(point Vec2) bool {
	return b.Rect.ContainsPoint(point)
}

func (b AABB) Distance(point Vec2) float64 {
	return point.DistanceTo(b.Rect.ClampPoint(point))
}

func rectPenetration(a, b Rect) (x, y float64) {
	x = math.Min(a.Max.X, b.Max.X) - math.Max(a.Min.X, b.Min.X)
	y = math.Min(a.Max.Y, b.Max.Y) - math.Max(a.Min.Y, b.Min.Y)
	return x, y
}

func shapesIntersect(a, b Shape) bool {
	switch first := a.(type) {
	case Circle:
		switch second := b.(type) {
		case Circle:
			return first.Center.DistanceTo(second.Center) < first.Radius+second.Radius-epsilon
		case AABB:
			return circleBoxPenetration(first, second).depth > epsilon
		}
	case AABB:
		switch second := b.(type) {
		case Circle:
			return circleBoxPenetration(second, first).depth > epsilon
		case AABB:
			overlapX, overlapY := rectPenetration(first.Rect, second.Rect)
			return overlapX > epsilon && overlapY > epsilon
		}
	}
	return false
}

func circleBoxPenetration(circle Circle, box AABB) (result penetration) {
	closest := closestPoint(circle.Center, box.Rect)
	delta := circle.Center.Sub(closest)
	distance := delta.Length()

	if distance > epsilon {
		result.normal = delta.Scale(1 / distance)
		result.depth = circle.Radius - distance
		return result
	}

	left := circle.Center.X - box.Rect.Min.X
	right := box.Rect.Max.X - circle.Center.X
	top := circle.Center.Y - box.Rect.Min.Y
	bottom := box.Rect.Max.Y - circle.Center.Y

	nearest := math.Min(math.Min(left, right), math.Min(top, bottom))
	result.depth = nearest + circle.Radius
	switch nearest {
	case left:
		result.normal = Vec2{X: -1}
	case right:
		result.normal = Vec2{X: 1}
	case top:
		result.normal = Vec2{Y: -1}
	default:
		result.normal = Vec2{Y: 1}
	}
	return result
}

func closestPoint(point Vec2, rect Rect) Vec2 {
	return rect.ClampPoint(point)
}

type penetration struct {
	normal Vec2
	depth  float64
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
