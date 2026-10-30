function Websocket() {
    this.socket = null;
    this.events = {};
    this.closeHandler = null;

    this.MESSAGE = 1;
    this.JOIN = 2;
    this.JOIN_WAIT = 3;
    this.JOIN_SUCCESS = 4;
    this.JOIN_FAILURE = 5;
    this.BATTLE_START = 6;
    this.ROOM_START = 7;
    this.ROOM_INFO = 8;
    this.ROOM_CLOSE = 9;
    this.AUTHENTICATION = 10;
    this.FAILURE = 11;
    this.UPDATE = 100;
    this.PRIVATE = 110;
    this.INIT = 101;
    this.REMOVE = 102;
    this.PLAYER_KEY_DOWN = 200;
    this.PLAYER_MOUSE_DOWN = 201;
    this.PLAYER_MOUSE_MOVE = 202;
    this.PLAYER_KILL = 203;
    this.PLAYER_CHOICE = 204;
}

Websocket.prototype.init = function (wsEndpoint) {
    this.socket = new WebSocket(wsEndpoint);

    return new Promise((resolve, reject) => {
        let settled = false;

        this.socket.addEventListener('open', () => {
            settled = true;
            resolve(this.socket);
        });

        this.socket.addEventListener('error', (event) => {
            if (!settled) {
                settled = true;
                reject(event);
            }
        });

        this.socket.addEventListener('close', (event) => {
            if (this.closeHandler) {
                this.closeHandler(event);
            }
            if (!settled) {
                settled = true;
                reject(event);
            }
        });

        this.socket.addEventListener('message', (event) => {
            let message;
            try {
                message = JSON.parse(event.data);
            } catch (err) {
                console.warn('Invalid server message', event.data);
                return;
            }

            const handler = this.events[message.type];
            if (typeof handler === 'function') {
                handler(message.data);
            }
        });
    });
};

Websocket.prototype.on = function (type, handler) {
    this.events[type] = handler;
};

Websocket.prototype.onClose = function (handler) {
    this.closeHandler = handler;
};

Websocket.prototype.isOpen = function () {
    return this.socket !== null && this.socket.readyState === WebSocket.OPEN;
};

Websocket.prototype.send = function (type, data) {
    if (!this.isOpen()) {
        return false;
    }

    const event = {type: type};
    if (data !== undefined) {
        event.data = data;
    }
    this.socket.send(JSON.stringify(event));
    return true;
};

Websocket.prototype.close = function (code, reason) {
    if (this.socket !== null &&
        (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
        this.socket.close(code, reason);
    }
};
