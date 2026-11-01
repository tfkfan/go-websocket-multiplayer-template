package player

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/tfkfan/gogame/internal/core/spatial"
)

type Controller interface {
	Direction() (float64, float64)
}

type KeyController interface {
	ApplyKey(key string, pressed bool) bool
}

type HumanController struct {
	keys atomic.Uint32
}

func NewHumanController() *HumanController {
	return &HumanController{}
}

func (c *HumanController) Direction() (float64, float64) {
	mask := c.keys.Load()

	dx, dy := 0.0, 0.0
	if mask&keyLeft != 0 {
		dx--
	}
	if mask&keyRight != 0 {
		dx++
	}
	if mask&keyUp != 0 {
		dy--
	}
	if mask&keyDown != 0 {
		dy++
	}
	if dx != 0 && dy != 0 {
		dx /= math.Sqrt2
		dy /= math.Sqrt2
	}
	return dx, dy
}

func (c *HumanController) ApplyKey(key string, pressed bool) bool {
	bit, ok := keyBits[key]
	if !ok {
		return false
	}

	for {
		old := c.keys.Load()
		next := old &^ bit
		if pressed {
			next = old | bit
		}
		if c.keys.CompareAndSwap(old, next) {
			return true
		}
	}
}

type ScriptedController struct {
	mu        sync.RWMutex
	direction spatial.Vec2
}

func NewScriptedController() *ScriptedController {
	return &ScriptedController{}
}

func (c *ScriptedController) SetDirection(x, y float64) {
	c.mu.Lock()
	c.direction = spatial.Vec2{X: x, Y: y}
	c.mu.Unlock()
}

func (c *ScriptedController) Direction() (float64, float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.direction.X, c.direction.Y
}
