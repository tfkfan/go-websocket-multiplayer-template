extends Node

signal room_created(room_id: String)
signal room_create_failed(message: String)
signal opened
signal join_received(data: Dictionary)
signal update_received(data: Dictionary)
signal closed(code: int, reason: String)
signal transport_error(message: String)

const ROOM_CREATE_TIMEOUT := 10.0

var _socket: WebSocketPeer = null
var _http: HTTPRequest = null
var _opened_emitted := false
var _creating_room := false

func _ready() -> void:
	_http = HTTPRequest.new()
	_http.timeout = ROOM_CREATE_TIMEOUT
	_http.request_completed.connect(_on_room_created_response)
	add_child(_http)

func _process(_delta: float) -> void:
	if _socket == null:
		return
	_socket.poll()
	match _socket.get_ready_state():
		WebSocketPeer.STATE_OPEN:
			if not _opened_emitted:
				_opened_emitted = true
				opened.emit()
			_drain_packets()
		WebSocketPeer.STATE_CLOSED:
			var code := _socket.get_close_code()
			var reason := _socket.get_close_reason()
			_socket = null
			_opened_emitted = false
			closed.emit(code, reason)

func create_room() -> void:
	if _creating_room or _http == null:
		return
	_creating_room = true
	var error := _http.request(Platform.http_origin() + Protocol.ROOM_ENDPOINT, PackedStringArray(), HTTPClient.METHOD_POST)
	if error != OK:
		_creating_room = false
		room_create_failed.emit("request could not be sent")

func connect_queue() -> void:
	_connect("")

func connect_room(room_id: String) -> void:
	_connect("?room=" + room_id.uri_encode())

func _connect(query: String) -> void:
	close_connection()
	_socket = WebSocketPeer.new()
	_opened_emitted = false
	var url := Platform.websocket_origin() + Protocol.WEB_SOCKET_ENDPOINT + query
	var error := _socket.connect_to_url(url)
	if error != OK:
		_socket = null
		transport_error.emit("the socket could not be opened")

func close_connection() -> void:
	if _socket == null:
		return
	var peer := _socket
	_socket = null
	_opened_emitted = false
	if peer.get_ready_state() != WebSocketPeer.STATE_CLOSED:
		peer.close(1000, "leaving")

func is_open() -> bool:
	return _socket != null and _socket.get_ready_state() == WebSocketPeer.STATE_OPEN

func send_key(direction: String, pressed: bool) -> void:
	_send(Protocol.PLAYER_KEY_DOWN, {"key": direction, "state": pressed})

func _send(message_type: int, data: Variant) -> void:
	if not is_open():
		return
	_socket.send_text(JSON.stringify({"type": message_type, "data": data}))

func _on_room_created_response(result: int, response_code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	_creating_room = false
	if result != HTTPRequest.RESULT_SUCCESS:
		room_create_failed.emit("the request did not complete")
		return
	if response_code != 200:
		room_create_failed.emit("the server returned " + str(response_code))
		return
	var room_id := body.get_string_from_utf8().strip_edges()
	if room_id.is_empty():
		room_create_failed.emit("the server returned an empty room id")
		return
	room_created.emit(room_id)

func _drain_packets() -> void:
	while _socket != null and _socket.get_available_packet_count() > 0:
		var packet := _socket.get_packet()
		if not _socket.was_string_packet():
			continue
		var parsed = JSON.parse_string(packet.get_string_from_utf8())
		if not (parsed is Dictionary):
			continue
		var envelope := parsed as Dictionary
		var message_type := int(envelope.get("type", 0))
		var data = envelope.get("data", null)
		match message_type:
			Protocol.JOIN_SUCCESS:
				if data is Dictionary:
					join_received.emit(data as Dictionary)
			Protocol.UPDATE:
				if data is Dictionary:
					update_received.emit(data as Dictionary)
			Protocol.FAILURE:
				if data is Dictionary:
					transport_error.emit(str(data.get("message", "Server failure")))
