class_name GameField
extends Control

const WORLD_WIDTH := 1200.0
const WORLD_HEIGHT := 800.0
const GRID_STEP := 80.0
const BORDER_WIDTH := 4.0
const DEFAULT_RADIUS := 25.0
const BACKGROUND_COLOR := Color("#0c1522")
const GRID_COLOR := Color(90.0 / 255.0, 135.0 / 255.0, 175.0 / 255.0, 0.13)
const BORDER_COLOR := Color("#37516d")
const MUTED_TEXT_COLOR := Color("#7c93ad")
const SELF_OUTLINE_COLOR := Color("#f8fafc")
const OTHER_OUTLINE_COLOR := Color(5.0 / 255.0, 12.0 / 255.0, 22.0 / 255.0, 0.8)
const SELF_FALLBACK_COLOR := Color("#38bdf8")
const OTHER_FALLBACK_COLOR := Color("#e74c3c")

var self_id := ""
var room_active := false

var _world_width := WORLD_WIDTH
var _world_height := WORLD_HEIGHT
var _players: Array = []
var _tick := 0
var _has_state := false

func _ready() -> void:
	resized.connect(_on_resized)

func _on_resized() -> void:
	queue_redraw()

func set_self_id(player_id: String) -> void:
	self_id = player_id
	queue_redraw()

func set_room_active(active: bool) -> void:
	room_active = active
	queue_redraw()

func set_world_size(width: float, height: float) -> void:
	if width <= 0.0 or height <= 0.0:
		return
	_world_width = width
	_world_height = height
	queue_redraw()

func apply_update(data: Dictionary) -> void:
	set_world_size(float(data.get("width", _world_width)), float(data.get("height", _world_height)))
	var players = data.get("players", [])
	_players = players if players is Array else []
	_tick = int(data.get("tick", 0))
	_has_state = true
	queue_redraw()

func clear_state() -> void:
	_players = []
	_tick = 0
	_has_state = false
	queue_redraw()

func has_state() -> bool:
	return _has_state

func player_count() -> int:
	return _players.size()

func latest_tick() -> int:
	return _tick

func player_position(player_id: String) -> Variant:
	for entry in _players:
		if not (entry is Dictionary):
			continue
		var player := entry as Dictionary
		if str(player.get("id", "")) != player_id:
			continue
		var position = player.get("position", null)
		if position is Dictionary:
			var point := position as Dictionary
			return Vector2(float(point.get("x", 0.0)), float(point.get("y", 0.0)))
	return null

func _draw() -> void:
	draw_rect(Rect2(Vector2.ZERO, size), BACKGROUND_COLOR)
	if size.x <= 0.0 or size.y <= 0.0:
		return
	var factor := minf(size.x / _world_width, size.y / _world_height)
	if factor <= 0.0:
		return
	var origin := Vector2((size.x - _world_width * factor) * 0.5, (size.y - _world_height * factor) * 0.5)
	var field_size := Vector2(_world_width * factor, _world_height * factor)
	_draw_grid(origin, field_size, factor)
	draw_rect(Rect2(origin, field_size), BORDER_COLOR, false, maxf(1.0, BORDER_WIDTH * factor))
	for entry in _players:
		_draw_player(entry, origin, factor)
	if not _has_state:
		_draw_empty_state(factor)

func _draw_grid(origin: Vector2, field_size: Vector2, factor: float) -> void:
	var x := GRID_STEP
	while x < _world_width:
		var px := origin.x + x * factor
		draw_line(Vector2(px, origin.y), Vector2(px, origin.y + field_size.y), GRID_COLOR, 1.0)
		x += GRID_STEP
	var y := GRID_STEP
	while y < _world_height:
		var py := origin.y + y * factor
		draw_line(Vector2(origin.x, py), Vector2(origin.x + field_size.x, py), GRID_COLOR, 1.0)
		y += GRID_STEP

func _draw_player(entry: Variant, origin: Vector2, factor: float) -> void:
	if not (entry is Dictionary):
		return
	var player := entry as Dictionary
	var position = player.get("position", null)
	if not (position is Dictionary):
		return
	var point := position as Dictionary
	var center := origin + Vector2(float(point.get("x", 0.0)), float(point.get("y", 0.0))) * factor
	var radius := maxf(1.0, float(player.get("radius", DEFAULT_RADIUS)) * factor)
	var is_self := not self_id.is_empty() and str(player.get("id", "")) == self_id
	draw_circle(center, radius, _player_color(player, is_self))
	var outline_color := SELF_OUTLINE_COLOR if is_self else OTHER_OUTLINE_COLOR
	var outline_width := (5.0 if is_self else 2.0) * clampf(factor, 0.5, 2.0)
	draw_arc(center, radius, 0.0, TAU, 64, outline_color, outline_width, true)
	if is_self:
		_draw_self_label(center, radius, factor)

func _player_color(player: Dictionary, is_self: bool) -> Color:
	var value := str(player.get("color", ""))
	if not value.is_empty() and Color.html_is_valid(value):
		return Color(value)
	return SELF_FALLBACK_COLOR if is_self else OTHER_FALLBACK_COLOR

func _draw_self_label(center: Vector2, radius: float, factor: float) -> void:
	var font := ThemeDB.fallback_font
	var font_size := maxi(10, int(16.0 * clampf(factor, 0.6, 2.0)))
	var label_width := 100.0
	var label_position := Vector2(center.x - label_width * 0.5, center.y - radius - 10.0 * factor)
	draw_string(font, label_position, "you", HORIZONTAL_ALIGNMENT_CENTER, label_width, font_size, SELF_OUTLINE_COLOR)

func _draw_empty_state(factor: float) -> void:
	var message := "Waiting for game state…" if room_active else "Create a room, open a link, or press Play"
	var font := ThemeDB.fallback_font
	var font_size := maxi(12, int(22.0 * clampf(factor, 0.5, 2.0)))
	draw_string(font, Vector2(0.0, size.y * 0.5), message, HORIZONTAL_ALIGNMENT_CENTER, size.x, font_size, MUTED_TEXT_COLOR)
