extends Control

const WIDE_MIN_WIDTH := 900.0
const PANEL_WIDTH := 320.0
const WIDE_MARGIN := 18.0
const NARROW_MARGIN := 12.0
const WIDE_GAP := 18.0
const NARROW_GAP := 12.0
const SHORT_ID_LENGTH := 8
const DIRECTIONS := ["UP", "DOWN", "LEFT", "RIGHT"]

@onready var stage: Control = %Stage
@onready var panel: PanelContainer = %Panel
@onready var game_field: GameField = %GameField
@onready var title: Label = %Title
@onready var subtitle: Label = %Subtitle
@onready var create_button: Button = %CreateButton
@onready var play_button: Button = %PlayButton
@onready var room_row: HBoxContainer = %RoomRow
@onready var room_label: Label = %RoomLabel
@onready var room_link: LinkButton = %RoomLink
@onready var copy_button: Button = %CopyButton
@onready var leave_button: Button = %LeaveButton
@onready var status: Label = %Status
@onready var state_header: Label = %StateHeader
@onready var stats: Label = %Stats
@onready var controls_header: Label = %ControlsHeader
@onready var controls_help: Label = %ControlsHelp

var _room_id := ""
var _self_id := ""
var _connected := false
var _queueing := false
var _leaving := false
var _direction_states := {"UP": false, "DOWN": false, "LEFT": false, "RIGHT": false}
var _direction_suppressed := {"UP": false, "DOWN": false, "LEFT": false, "RIGHT": false}

func _ready() -> void:
	_apply_theme()
	_connect_signals()
	_set_room_visible(false)
	_set_status("Create a room or press Play.", false)
	_update_stats()
	Platform.register_visibility_callback(_on_focus_lost)
	var requested_room := Platform.room_from_query()
	if not requested_room.is_empty():
		_show_room(requested_room)
		_connect_room()
	resized.connect(_layout)
	_layout.call_deferred()

func _process(_delta: float) -> void:
	_set_direction(Protocol.DIRECTION_UP, Input.is_physical_key_pressed(KEY_W) or Input.is_physical_key_pressed(KEY_UP))
	_set_direction(Protocol.DIRECTION_DOWN, Input.is_physical_key_pressed(KEY_S) or Input.is_physical_key_pressed(KEY_DOWN))
	_set_direction(Protocol.DIRECTION_LEFT, Input.is_physical_key_pressed(KEY_A) or Input.is_physical_key_pressed(KEY_LEFT))
	_set_direction(Protocol.DIRECTION_RIGHT, Input.is_physical_key_pressed(KEY_D) or Input.is_physical_key_pressed(KEY_RIGHT))

func _notification(what: int) -> void:
	if what == NOTIFICATION_APPLICATION_FOCUS_OUT:
		_release_all_directions()

func _apply_theme() -> void:
	panel.add_theme_stylebox_override("panel", UiTheme.panel_style())
	title.add_theme_font_size_override("font_size", 24)
	title.add_theme_color_override("font_color", UiTheme.TEXT)
	subtitle.add_theme_font_size_override("font_size", 13)
	subtitle.add_theme_color_override("font_color", UiTheme.MUTED)
	state_header.add_theme_font_size_override("font_size", 11)
	state_header.add_theme_color_override("font_color", UiTheme.MUTED)
	controls_header.add_theme_font_size_override("font_size", 11)
	controls_header.add_theme_color_override("font_color", UiTheme.MUTED)
	controls_help.add_theme_font_size_override("font_size", 13)
	controls_help.add_theme_color_override("font_color", UiTheme.MUTED)
	room_label.add_theme_font_size_override("font_size", 11)
	room_label.add_theme_color_override("font_color", UiTheme.MUTED)
	UiTheme.apply_link(room_link)
	UiTheme.apply_button(create_button, &"primary")
	UiTheme.apply_button(play_button, &"primary")
	UiTheme.apply_button(copy_button, &"ghost")
	UiTheme.apply_button(leave_button, &"danger")
	status.add_theme_font_size_override("font_size", 13)
	stats.add_theme_font_size_override("font_size", 12)
	stats.add_theme_color_override("font_color", UiTheme.STATS_TEXT)
	stats.add_theme_stylebox_override("normal", UiTheme.statistics_style())
	status.add_theme_stylebox_override("normal", UiTheme.status_style(false))

func _connect_signals() -> void:
	create_button.pressed.connect(_on_create_pressed)
	play_button.pressed.connect(_on_play_pressed)
	copy_button.pressed.connect(_on_copy_pressed)
	leave_button.pressed.connect(_on_leave_pressed)
	Network.room_created.connect(_on_room_created)
	Network.room_create_failed.connect(_on_room_create_failed)
	Network.opened.connect(_on_opened)
	Network.join_received.connect(_on_join_received)
	Network.update_received.connect(_on_update_received)
	Network.closed.connect(_on_closed)
	Network.transport_error.connect(_on_transport_error)

func _on_create_pressed() -> void:
	create_button.disabled = true
	play_button.disabled = true
	_set_status("Creating room…", false)
	Network.create_room()

func _on_play_pressed() -> void:
	play_button.disabled = true
	create_button.visible = false
	play_button.visible = false
	room_row.visible = false
	leave_button.visible = true
	copy_button.visible = false
	_queueing = true
	_connected = false
	_room_id = ""
	_self_id = ""
	game_field.set_room_active(true)
	game_field.clear_state()
	_set_status("Finding players…", false)
	_update_stats()
	Network.connect_queue()

func _on_room_created(room_id: String) -> void:
	create_button.disabled = false
	_show_room(room_id)
	Platform.set_room_in_address(room_id)
	_connect_room()

func _on_room_create_failed(message: String) -> void:
	create_button.disabled = false
	play_button.disabled = false
	_set_status("Could not create room: " + message, true)

func _on_opened() -> void:
	_connected = true
	if _queueing:
		_set_status("Waiting for players…", false)
		return
	_set_status("Connected. Waiting for the game state…", false)

func _on_join_received(data: Dictionary) -> void:
	if _queueing:
		_queueing = false
		_room_id = str(data.get("roomId", ""))
		if not _room_id.is_empty():
			Platform.set_room_in_address(_room_id)
			room_link.text = _room_id
			room_link.uri = Platform.room_url(_room_id)
			room_row.visible = true
			copy_button.visible = true
		create_button.visible = false
		play_button.visible = false
		leave_button.visible = true
	var player_id := str(data.get("playerId", ""))
	if not player_id.is_empty():
		_self_id = player_id
		game_field.set_self_id(player_id)
	game_field.set_world_size(float(data.get("width", 0.0)), float(data.get("height", 0.0)))
	var tick_rate := int(data.get("tickRate", 0))
	_set_status("In room " + _short_id(_room_id) + " · " + str(tick_rate) + " updates/s", false)
	_update_stats()

func _on_update_received(data: Dictionary) -> void:
	_connected = true
	game_field.apply_update(data)
	_update_stats()

func _on_closed(code: int, reason: String) -> void:
	_connected = false
	_reset_direction_state()
	game_field.clear_state()
	_update_stats()
	if _queueing:
		_queueing = false
		_reset_room()
		_set_status("Left the queue.", false)
		return
	if _leaving:
		_leaving = false
		_set_status("Left the room.", false)
		return
	if code == 1001:
		_reset_room()
		_set_status("Room closed.", true)
		return
	var detail := ""
	if code > 0:
		detail = " (" + str(code) + ")"
	if not reason.is_empty():
		detail += ": " + reason
	_set_status("Disconnected" + detail, true)
	create_button.visible = true
	create_button.text = "Create new room"
	leave_button.visible = false

func _on_transport_error(message: String) -> void:
	_set_status(message, true)

func _on_copy_pressed() -> void:
	if _room_id.is_empty():
		return
	Platform.copy_text(Platform.room_url(_room_id), _on_copy_finished)

func _on_copy_finished(succeeded: bool) -> void:
	_set_status("Room link copied." if succeeded else "Could not copy the room link.", not succeeded)

func _on_leave_pressed() -> void:
	_release_all_directions()
	_leaving = true
	_connected = false
	Network.close_connection()
	_reset_room()
	_set_status("Left the room.", false)

func _on_focus_lost(_arguments: Array) -> void:
	_release_all_directions()

func _show_room(room_id: String) -> void:
	_room_id = room_id
	room_link.text = room_id
	room_link.uri = Platform.room_url(room_id)
	room_row.visible = true
	create_button.visible = false
	play_button.visible = false
	create_button.disabled = false
	leave_button.visible = true
	copy_button.visible = true
	game_field.set_room_active(true)
	game_field.clear_state()
	_update_stats()

func _reset_room() -> void:
	_room_id = ""
	_self_id = ""
	_connected = false
	room_row.visible = false
	create_button.visible = true
	create_button.disabled = false
	create_button.text = "Create room"
	play_button.visible = true
	play_button.disabled = false
	_queueing = false
	leave_button.visible = false
	copy_button.visible = false
	game_field.set_self_id("")
	game_field.set_room_active(false)
	game_field.clear_state()
	Platform.set_room_in_address("")
	_update_stats()

func _set_room_visible(visible: bool) -> void:
	room_row.visible = visible
	create_button.visible = not visible
	play_button.visible = not visible
	leave_button.visible = visible
	copy_button.visible = visible

func _connect_room() -> void:
	if _room_id.is_empty():
		return
	_leaving = false
	_self_id = ""
	game_field.set_self_id("")
	game_field.set_room_active(true)
	_set_status("Connecting to room " + _short_id(_room_id) + "…", false)
	Network.connect_room(_room_id)

func _set_direction(direction: String, pressed: bool) -> void:
	if bool(_direction_suppressed.get(direction, false)):
		if pressed:
			return
		_direction_suppressed[direction] = false
		return
	if bool(_direction_states.get(direction, false)) == pressed:
		return
	_direction_states[direction] = pressed
	if _connected:
		Network.send_key(direction, pressed)

func _release_all_directions() -> void:
	for direction in DIRECTIONS:
		_set_direction(direction, false)
		_direction_suppressed[direction] = true

func _reset_direction_state() -> void:
	for direction in DIRECTIONS:
		_direction_states[direction] = false
		_direction_suppressed[direction] = true

func _layout() -> void:
	if size.x <= 0.0 or size.y <= 0.0:
		return
	if size.x >= WIDE_MIN_WIDTH:
		var wide_stage_width := maxf(0.0, size.x - WIDE_MARGIN * 2.0 - WIDE_GAP - PANEL_WIDTH)
		var wide_stage_height := maxf(0.0, size.y - WIDE_MARGIN * 2.0)
		_place(stage, WIDE_MARGIN, WIDE_MARGIN, wide_stage_width, wide_stage_height)
		_place(panel, WIDE_MARGIN + wide_stage_width + WIDE_GAP, WIDE_MARGIN, PANEL_WIDTH, wide_stage_height)
	else:
		var content_width := maxf(0.0, size.x - NARROW_MARGIN * 2.0)
		var narrow_stage_height := content_width * 2.0 / 3.0
		_place(stage, NARROW_MARGIN, NARROW_MARGIN, content_width, narrow_stage_height)
		var panel_top := NARROW_MARGIN + narrow_stage_height + NARROW_GAP
		_place(panel, NARROW_MARGIN, panel_top, content_width, maxf(0.0, size.y - panel_top - NARROW_MARGIN))

func _place(control: Control, x: float, y: float, width: float, height: float) -> void:
	control.position = Vector2(x, y)
	control.size = Vector2(width, height)

func _update_stats() -> void:
	if _room_id.is_empty() or not game_field.has_state():
		stats.text = "No game state yet."
		return
	var position: Variant = game_field.player_position(_self_id)
	var position_text := "—"
	if position is Vector2:
		var point := position as Vector2
		position_text = str(int(roundf(point.x))) + ", " + str(int(roundf(point.y)))
	var lines := PackedStringArray()
	lines.append("room    " + _short_id(_room_id))
	lines.append("players " + str(game_field.player_count()))
	lines.append("you     " + _short_id(_self_id))
	lines.append("pos     " + position_text)
	lines.append("tick    " + str(game_field.latest_tick()))
	stats.text = "\n".join(lines)

func _short_id(value: String) -> String:
	if value.is_empty():
		return "—"
	var shortened := value.substr(0, SHORT_ID_LENGTH)
	if value.length() > SHORT_ID_LENGTH:
		shortened += "…"
	return shortened

func _set_status(message: String, is_error: bool) -> void:
	status.text = message
	status.add_theme_color_override("font_color", UiTheme.ERROR_TEXT if is_error else UiTheme.STATUS_TEXT)
	status.add_theme_stylebox_override("normal", UiTheme.status_style(is_error))
