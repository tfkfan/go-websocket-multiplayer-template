package player

import (
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/core/entity"
	"github.com/tfkfan/gogame/internal/spatial"
)

func TestProfilesMoveIndependently(t *testing.T) {
	world := entity.World{Width: 1200, Height: 800}

	fastProfile := DefaultProfile()
	fastProfile.Speed = 300
	fastProfile.Radius = 30
	fastProfile.Color = "#111111"

	slowProfile := DefaultProfile()
	slowProfile.Speed = 100
	slowProfile.Radius = 20
	slowProfile.Color = "#222222"

	fastController := NewScriptedController()
	fastController.SetDirection(1, 0)
	slowController := NewScriptedController()
	slowController.SetDirection(1, 0)

	fast := New(uuid.New(), fastProfile, fastController)
	slow := New(uuid.New(), slowProfile, slowController)
	fast.Spawn(world, spatial.Vec2{X: 100, Y: 100}, fastProfile.Color)
	slow.Spawn(world, spatial.Vec2{X: 100, Y: 100}, slowProfile.Color)

	fast.Update(1)
	slow.Update(1)

	fastState := fast.Snapshot()
	slowState := slow.Snapshot()

	if math.Abs(fastState.Position.X-400) > 1e-9 {
		t.Fatalf("fast player x = %g, want 400", fastState.Position.X)
	}
	if math.Abs(slowState.Position.X-200) > 1e-9 {
		t.Fatalf("slow player x = %g, want 200", slowState.Position.X)
	}
	if fastState.Radius != 30 || slowState.Radius != 20 {
		t.Fatalf("radii = %g and %g, want 30 and 20", fastState.Radius, slowState.Radius)
	}
	if fastState.Color != "#111111" || slowState.Color != "#222222" {
		t.Fatalf("colors = %q and %q, want per-profile colors", fastState.Color, slowState.Color)
	}
}

func TestScriptedControllerMovesLikeKeyboard(t *testing.T) {
	world := entity.World{Width: 1200, Height: 800}
	profile := DefaultProfile()

	human := New(uuid.New(), profile, NewHumanController())
	human.ApplyKey("RIGHT", true)
	human.Spawn(world, spatial.Vec2{X: 100, Y: 100}, "")

	scripted := New(uuid.New(), profile, NewScriptedController())
	scripted.controller.(*ScriptedController).SetDirection(1, 0)
	scripted.Spawn(world, spatial.Vec2{X: 100, Y: 100}, "")

	human.Update(0.5)
	scripted.Update(0.5)

	if human.Position != scripted.Position {
		t.Fatalf("human moved to %+v, scripted moved to %+v", human.Position, scripted.Position)
	}
}

func TestHumanControllerDirection(t *testing.T) {
	controller := NewHumanController()

	if controller.ApplyKey("UNKNOWN", true) {
		t.Fatal("unknown key was accepted")
	}
	if !controller.ApplyKey("UP", true) {
		t.Fatal("UP key was rejected")
	}
	if dx, dy := controller.Direction(); dx != 0 || dy != -1 {
		t.Fatalf("UP direction = (%g, %g), want (0, -1)", dx, dy)
	}

	controller.ApplyKey("RIGHT", true)
	dx, dy := controller.Direction()
	if math.Abs(dx-math.Sqrt2/2) > 1e-9 || math.Abs(dy+math.Sqrt2/2) > 1e-9 {
		t.Fatalf("diagonal direction = (%g, %g), want normalized", dx, dy)
	}

	controller.ApplyKey("UP", false)
	controller.ApplyKey("RIGHT", false)
	if dx, dy := controller.Direction(); dx != 0 || dy != 0 {
		t.Fatalf("released direction = (%g, %g), want (0, 0)", dx, dy)
	}
}
