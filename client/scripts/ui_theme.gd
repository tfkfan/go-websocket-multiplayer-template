class_name UiTheme
extends RefCounted

const BACKGROUND := Color("#070c14")
const PANEL := Color("#101a27")
const PANEL_DEEP := Color("#0c1521")
const LINE := Color("#263a50")
const TEXT := Color("#e8eef7")
const MUTED := Color("#8aa0b8")
const ACCENT := Color("#38bdf8")
const DANGER := Color("#f87171")
const FIELD := Color("#0c1522")
const STATS_TEXT := Color("#b7c9dd")
const STATUS_TEXT := Color("#c9e8f8")
const ERROR_TEXT := Color("#fecaca")
const PRIMARY_DARK := Color("#04121f")
const BUTTON := Color("#1a2a3c")
const BUTTON_HOVER := Color("#20354b")
const BUTTON_PRESSED := Color("#162333")
const BUTTON_BORDER_HOVER := Color("#3d5d7d")
const PRIMARY := Color("#0ea5e9")
const PRIMARY_PRESSED := Color("#0284c7")

static func panel_style() -> StyleBoxFlat:
	var style := _box(PANEL, LINE, 14, 0)
	style.set_content_margin_all(0.0)
	style.shadow_color = Color(0.0, 0.0, 0.0, 0.35)
	style.shadow_size = 16
	return style

static func status_style(is_error: bool) -> StyleBoxFlat:
	var style := StyleBoxFlat.new()
	style.bg_color = Color(0.972549, 0.443137, 0.443137, 0.09) if is_error else Color(0.219608, 0.741176, 0.972549, 0.08)
	style.border_color = DANGER if is_error else ACCENT
	style.border_width_left = 3
	style.content_margin_left = 11.0
	style.content_margin_right = 11.0
	style.content_margin_top = 10.0
	style.content_margin_bottom = 10.0
	return style

static func statistics_style() -> StyleBoxFlat:
	return _box(PANEL_DEEP, LINE, 9, 11)

static func apply_button(button: Button, kind: StringName) -> void:
	var normal := _box(BUTTON, LINE, 9, 9)
	var hover := _box(BUTTON_HOVER, BUTTON_BORDER_HOVER, 9, 9)
	var pressed := _box(BUTTON_PRESSED, LINE, 9, 9)
	var font_color := TEXT
	match kind:
		&"primary":
			normal = _box(PRIMARY, ACCENT, 9, 9)
			hover = _box(ACCENT, ACCENT, 9, 9)
			pressed = _box(PRIMARY_PRESSED, ACCENT, 9, 9)
			font_color = PRIMARY_DARK
		&"danger":
			font_color = DANGER
		&"ghost":
			normal = _box(Color(0.0, 0.0, 0.0, 0.0), LINE, 9, 5)
			hover = _box(BUTTON_HOVER, LINE, 9, 5)
			pressed = _box(BUTTON_PRESSED, LINE, 9, 5)
	button.add_theme_stylebox_override("normal", normal)
	button.add_theme_stylebox_override("hover", hover)
	button.add_theme_stylebox_override("pressed", pressed)
	button.add_theme_stylebox_override("disabled", normal)
	button.add_theme_color_override("font_color", font_color)
	button.add_theme_color_override("font_hover_color", font_color)
	button.add_theme_color_override("font_pressed_color", font_color)
	button.add_theme_color_override("font_disabled_color", font_color)

static func apply_link(link: LinkButton) -> void:
	link.add_theme_color_override("font_color", ACCENT)
	link.add_theme_color_override("font_hover_color", TEXT)
	link.add_theme_color_override("font_pressed_color", TEXT)
	link.add_theme_font_size_override("font_size", 12)

static func _box(background: Color, border: Color, radius: int, padding: int) -> StyleBoxFlat:
	var style := StyleBoxFlat.new()
	style.bg_color = background
	style.border_color = border
	style.set_border_width_all(1)
	style.set_corner_radius_all(radius)
	style.content_margin_left = float(padding + 4)
	style.content_margin_right = float(padding + 4)
	style.content_margin_top = float(padding)
	style.content_margin_bottom = float(padding)
	return style
