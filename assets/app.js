(() => {
    'use strict';

    const WS_ENDPOINT = '/ws';
    const ROOM_ENDPOINT = '/room';
    const KEY_MAP = {
        KeyW: 'UP',
        ArrowUp: 'UP',
        KeyS: 'DOWN',
        ArrowDown: 'DOWN',
        KeyA: 'LEFT',
        ArrowLeft: 'LEFT',
        KeyD: 'RIGHT',
        ArrowRight: 'RIGHT'
    };

    const canvas = document.getElementById('game');
    const ctx = canvas.getContext('2d');
    const createBtn = document.getElementById('createRoomBtn');
    const copyBtn = document.getElementById('copyRoomBtn');
    const leaveBtn = document.getElementById('leaveRoomBtn');
    const roomRow = document.getElementById('roomRow');
    const roomLink = document.getElementById('roomLink');
    const statusEl = document.getElementById('status');
    const statsEl = document.getElementById('stats');

    const ws = new Websocket();
    const pressed = new Set();

    let roomId = null;
    let selfId = null;
    let state = null;
    let connected = false;
    let leaving = false;

    function roomURL(id) {
        const url = new URL(location.href);
        url.search = '';
        url.hash = '';
        url.searchParams.set('room', id);
        return url.toString();
    }

    function shortId(id) {
        return id ? id.slice(0, 8) + '…' : '—';
    }

    function setStatus(text, isError) {
        statusEl.textContent = text;
        statusEl.classList.toggle('error', Boolean(isError));
    }

    function showRoom(id) {
        roomId = id;
        roomLink.textContent = id;
        roomLink.href = roomURL(id);
        roomRow.hidden = false;
        createBtn.hidden = true;
        leaveBtn.hidden = false;
        copyBtn.hidden = false;
    }

    function resetRoomUI() {
        roomId = null;
        selfId = null;
        state = null;
        roomRow.hidden = true;
        createBtn.hidden = false;
        createBtn.textContent = 'Create room';
        leaveBtn.hidden = true;
        copyBtn.hidden = true;
        history.replaceState(null, '', location.pathname);
    }

    function configureCanvas(width, height) {
        if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
            return;
        }
        if (canvas.width !== Math.round(width) || canvas.height !== Math.round(height)) {
            canvas.width = Math.round(width);
            canvas.height = Math.round(height);
            canvas.style.aspectRatio = width + ' / ' + height;
        }
    }

    function draw() {
        const width = canvas.width;
        const height = canvas.height;

        ctx.clearRect(0, 0, width, height);
        ctx.fillStyle = '#0c1522';
        ctx.fillRect(0, 0, width, height);

        ctx.strokeStyle = 'rgba(90, 135, 175, 0.13)';
        ctx.lineWidth = 1;
        const grid = 80;
        for (let x = grid; x < width; x += grid) {
            ctx.beginPath();
            ctx.moveTo(x, 0);
            ctx.lineTo(x, height);
            ctx.stroke();
        }
        for (let y = grid; y < height; y += grid) {
            ctx.beginPath();
            ctx.moveTo(0, y);
            ctx.lineTo(width, y);
            ctx.stroke();
        }

        ctx.strokeStyle = '#37516d';
        ctx.lineWidth = 4;
        ctx.strokeRect(2, 2, width - 4, height - 4);

        if (!state || !Array.isArray(state.players)) {
            ctx.fillStyle = '#7c93ad';
            ctx.font = '600 22px system-ui, sans-serif';
            ctx.textAlign = 'center';
            ctx.fillText(roomId ? 'Waiting for game state…' : 'Create or open a room to play',
                width / 2, height / 2);
            ctx.textAlign = 'start';
            return;
        }

        for (const p of state.players) {
            if (!p || !p.position) {
                continue;
            }
            const isSelf = p.id === selfId;
            const radius = Number.isFinite(p.radius) ? p.radius : 25;

            ctx.beginPath();
            ctx.arc(p.position.x, p.position.y, radius, 0, Math.PI * 2);
            ctx.fillStyle = p.color || (isSelf ? '#38bdf8' : '#e74c3c');
            ctx.fill();
            ctx.lineWidth = isSelf ? 5 : 2;
            ctx.strokeStyle = isSelf ? '#f8fafc' : 'rgba(5, 12, 22, 0.8)';
            ctx.stroke();
        }

        const me = state.players.find((p) => p && p.id === selfId);
        if (me && me.position) {
            const radius = Number.isFinite(me.radius) ? me.radius : 25;
            ctx.fillStyle = '#f8fafc';
            ctx.font = '600 16px ui-monospace, monospace';
            ctx.textAlign = 'center';
            ctx.fillText('you', me.position.x, me.position.y - radius - 10);
            ctx.textAlign = 'start';
        }
    }

    function updateStats() {
        if (!state) {
            statsEl.textContent = 'No game state yet.';
            return;
        }

        const me = Array.isArray(state.players)
            ? state.players.find((p) => p && p.id === selfId)
            : null;
        const position = me && me.position
            ? Math.round(me.position.x) + ', ' + Math.round(me.position.y)
            : '—';

        statsEl.textContent = [
            'room    ' + shortId(roomId),
            'players ' + (Array.isArray(state.players) ? state.players.length : 0),
            'you     ' + shortId(selfId),
            'pos     ' + position,
            'tick    ' + (state.tick || 0)
        ].join('\n');
    }

    function press(key) {
        if (pressed.has(key)) {
            return;
        }
        pressed.add(key);
        if (connected) {
            ws.send(ws.PLAYER_KEY_DOWN, {key: key, state: true});
        }
    }

    function release(key) {
        if (!pressed.has(key)) {
            return;
        }
        pressed.delete(key);
        if (connected) {
            ws.send(ws.PLAYER_KEY_DOWN, {key: key, state: false});
        }
    }

    function releaseAllKeys() {
        for (const key of Array.from(pressed)) {
            release(key);
        }
    }

    function handleClose(event) {
        connected = false;
        releaseAllKeys();
        state = null;
        draw();
        updateStats();

        if (leaving) {
            leaving = false;
            setStatus('Left the room.');
            return;
        }

        if (event && event.code === 1001) {
            resetRoomUI();
            setStatus('Room closed.', true);
            draw();
            return;
        }

        const code = event && event.code ? ' (' + event.code + ')' : '';
        const reason = event && event.reason ? ': ' + event.reason : '';
        setStatus('Disconnected' + code + reason, true);
        createBtn.hidden = false;
        createBtn.textContent = 'Create new room';
        leaveBtn.hidden = true;
    }

    function connect() {
        if (!roomId) {
            return;
        }

        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = protocol + '//' + location.host + WS_ENDPOINT +
            '?room=' + encodeURIComponent(roomId);

        setStatus('Connecting to room ' + shortId(roomId) + '…');
        ws.init(url).then(() => {
            connected = true;
            setStatus('Connected. Waiting for the game state…');
        }).catch((event) => {
            connected = false;
            const reason = event && event.reason ? ': ' + event.reason : '';
            setStatus('Could not connect' + reason, true);
        });
    }

    async function createRoom() {
        createBtn.disabled = true;
        setStatus('Creating room…');

        try {
            const response = await fetch(ROOM_ENDPOINT, {method: 'POST'});
            if (!response.ok) {
                throw new Error('HTTP ' + response.status);
            }
            const id = (await response.text()).trim();
            if (!id) {
                throw new Error('empty room id');
            }

            showRoom(id);
            const url = new URL(location.href);
            url.search = '';
            url.searchParams.set('room', id);
            history.replaceState(null, '', url.toString());
            connect();
        } catch (err) {
            setStatus('Could not create room: ' + err.message, true);
        } finally {
            createBtn.disabled = false;
        }
    }

    async function copyRoomLink() {
        if (!roomId) {
            return;
        }
        try {
            await navigator.clipboard.writeText(roomURL(roomId));
            setStatus('Room link copied.');
        } catch (err) {
            setStatus('Could not copy the room link.', true);
        }
    }

    function leaveRoom() {
        releaseAllKeys();
        leaving = ws.isOpen();
        ws.close(1000, 'leaving');
        connected = false;
        resetRoomUI();
        draw();
        updateStats();
        setStatus('Left the room.');
    }

    ws.on(ws.JOIN_SUCCESS, (data) => {
        if (!data) {
            return;
        }
        selfId = data.playerId || selfId;
        configureCanvas(data.width, data.height);
        setStatus('In room ' + shortId(data.roomId) + ' · ' + data.tickRate + ' updates/s');
        updateStats();
    });

    ws.on(ws.UPDATE, (data) => {
        if (!data || !Array.isArray(data.players)) {
            return;
        }
        connected = true;
        state = data;
        configureCanvas(data.width, data.height);
        draw();
        updateStats();
    });

    ws.on(ws.FAILURE, (data) => {
        setStatus((data && data.message) || 'Server failure', true);
    });

    ws.onClose(handleClose);

    window.addEventListener('keydown', (event) => {
        const tag = event.target && event.target.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA') {
            return;
        }
        const key = KEY_MAP[event.code];
        if (!key) {
            return;
        }
        event.preventDefault();
        press(key);
    });

    window.addEventListener('keyup', (event) => {
        const key = KEY_MAP[event.code];
        if (!key) {
            return;
        }
        event.preventDefault();
        release(key);
    });

    window.addEventListener('blur', releaseAllKeys);
    document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
            releaseAllKeys();
        }
    });

    createBtn.addEventListener('click', createRoom);
    copyBtn.addEventListener('click', copyRoomLink);
    leaveBtn.addEventListener('click', leaveRoom);

    const requestedRoom = new URLSearchParams(location.search).get('room');
    if (requestedRoom) {
        showRoom(requestedRoom);
        connect();
    } else {
        setStatus('Create a room to start.');
    }

    draw();
    updateStats();
})();
