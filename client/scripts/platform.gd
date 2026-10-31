class_name Platform
extends RefCounted

const DESKTOP_HTTP_ORIGIN := "http://127.0.0.1:8080"
const DESKTOP_WEBSOCKET_ORIGIN := "ws://127.0.0.1:8080"

static var _visibility_callback = null
static var _copy_callback = null
static var _copy_window = null

static func is_web() -> bool:
	return OS.has_feature("web")

static func javascript_bridge() -> Object:
	if not is_web():
		return null
	return Engine.get_singleton("JavaScriptBridge")

static func eval_js(code: String) -> Variant:
	var bridge := javascript_bridge()
	if bridge == null:
		return null
	return bridge.call("eval", code)

static func http_origin() -> String:
	if is_web():
		var origin = eval_js("window.location.origin")
		if origin is String and not (origin as String).is_empty():
			return origin as String
	return DESKTOP_HTTP_ORIGIN

static func websocket_origin() -> String:
	if is_web():
		var origin = eval_js("(window.location.protocol === 'https:' ? 'wss://' : 'ws://') + window.location.host")
		if origin is String and not (origin as String).is_empty():
			return origin as String
	return DESKTOP_WEBSOCKET_ORIGIN

static func room_from_query() -> String:
	if not is_web():
		return ""
	var value = eval_js("new URLSearchParams(window.location.search).get('room')")
	if value is String:
		return (value as String).strip_edges()
	return ""

static func room_url(room_id: String) -> String:
	var base := http_origin() + "/"
	if is_web():
		var current = eval_js("window.location.origin + window.location.pathname")
		if current is String and not (current as String).is_empty():
			base = current as String
	if room_id.is_empty():
		return base
	return base + "?room=" + room_id.uri_encode()

static func set_room_in_address(room_id: String) -> void:
	if not is_web():
		return
	var path_value = eval_js("window.location.pathname")
	var target := "/" if not (path_value is String) else (path_value as String)
	if not room_id.is_empty():
		target += "?room=" + room_id.uri_encode()
	eval_js("history.replaceState(null, '', " + JSON.stringify(target) + ")")

static func document_hidden() -> bool:
	if not is_web():
		return false
	return bool(eval_js("document.hidden"))

static func register_visibility_callback(callback: Callable) -> void:
	if _visibility_callback != null:
		return
	var bridge := javascript_bridge()
	if bridge == null:
		return
	_visibility_callback = bridge.call("create_callback", callback)
	var window = bridge.call("get_interface", "window")
	if window != null:
		window.addEventListener("blur", _visibility_callback)
	var document = bridge.call("get_interface", "document")
	if document != null:
		document.addEventListener("visibilitychange", _visibility_callback)

static func copy_text(text: String, callback: Callable) -> void:
	var bridge := javascript_bridge()
	if bridge == null:
		DisplayServer.clipboard_set(text)
		callback.call(true)
		return
	if _copy_callback != null:
		return
	var window = bridge.call("get_interface", "window")
	if window == null:
		callback.call(false)
		return
	_copy_window = window
	_copy_callback = bridge.call("create_callback", func(arguments: Array) -> void:
		_copy_callback = null
		_copy_window = null
		var succeeded := arguments.size() > 0 and bool(arguments[0])
		callback.call(succeeded)
	)
	window.__godotCopyResult = _copy_callback
	eval_js("navigator.clipboard.writeText(" + JSON.stringify(text) + ").then(function () { window.__godotCopyResult(true); }, function () { window.__godotCopyResult(false); });")
