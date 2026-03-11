<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import './+page.css';

	interface Message {
		text: string;
		isSys: boolean;
		sender?: string;
		timestamp: Date;
		isMine: boolean;
	}

	interface Room {
		name: string;
		hasPass: boolean;
		userCount: number;
	}

	let ws: WebSocket | null = null;
	let currentRoom = '';
	let chatbox: HTMLElement;
	let myUsername: string = localStorage.getItem('chat_username') || '';
	let guestUsername = 'Guest' + Math.floor(Math.random() * 1000);
	let showLogin = true;
	let showRoomControls = false;
	let messages: Message[] = [];
	let messageInput = '';
	let roomList: Room[] = [];
	let showPasswordPrompt = false;
	let showDisclaimer = localStorage.getItem('disclaimer_seen') !== 'true';
	let pendingRoom = '';
	let pendingAction = '';
	let roomUserCount = 0;
	let refreshInterval: ReturnType<typeof setInterval>;
	const ROOMS_TOKEN = 'public-chat-token';
	const WS_URL = 'wss://temp-chat-production-45a1.up.railway.app';
	const API_URL = 'https://temp-chat-production-45a1.up.railway.app';

	$: if (chatbox && messages.length) {
		chatbox.scrollTop = chatbox.scrollHeight;
	}

	onMount(() => {
		if (myUsername) {
			showLogin = false;
			showRoomControls = true;
			fetchRooms();
			startRefresh();
		}
	});

	async function fetchRooms() {
		try {
			const res = await fetch(API_URL + '/rooms?token=' + ROOMS_TOKEN);
			if (!res.ok) return;
			const data: { rooms: Room[] } = await res.json();
			roomList = data.rooms || [];
		} catch (e) {
			console.error('Failed to fetch rooms', e);
		}
	}

	onDestroy(() => {
		if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	});

	function startRefresh() {
		refreshInterval = setInterval(() => {
			fetchRooms();
		}, 3000);
	}

	function login() {
		const usernameInput = document.getElementById('username') as HTMLInputElement;
		const username = usernameInput?.value?.trim();
		if (!username) {
			alert('Please enter a username');
			return;
		}
		localStorage.setItem('chat_username', username);
		myUsername = username;
		showLogin = false;
		showRoomControls = true;
		fetchRooms();
		startRefresh();
	}

	function logout() {
		if (ws) ws.close();
		ws = null;
		currentRoom = '';
		messages = [];
		localStorage.removeItem('chat_username');
		myUsername = '';
		showLogin = true;
		showRoomControls = false;
	}

	function saveMessages(room: string, msgs: Message[]) {
		localStorage.setItem('chat_' + room, JSON.stringify(msgs));
	}

	function loadMessages(room: string): Message[] {
		const stored = localStorage.getItem('chat_' + room);
		if (!stored) return [];
		const parsed: Message[] = JSON.parse(stored);
		return parsed.map((m) => ({
			...m,
			timestamp: new Date(m.timestamp)
		}));
	}

	function parseMessage(text: string): { sender?: string; text: string; isMine: boolean } {
		if (text.startsWith('[')) {
			const match = text.match(/^\[([^\]]+)\]\s*(.*)$/);
			if (match) {
				const sender = match[1];
				const msgText = match[2];
				const isMine = sender === (myUsername || guestUsername);
				return { sender, text: msgText, isMine };
			}
		}
		return { text, isMine: false };
	}

	function addMessage(text: string, isSys: boolean) {
		const {
			sender,
			text: msgText,
			isMine
		} = isSys ? { sender: undefined, text, isMine: false } : parseMessage(text);
		messages = [...messages, { text: msgText, isSys, sender, timestamp: new Date(), isMine }];
	}

	function formatTime(date: Date): string {
		return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	function promptJoinRoom(roomName: string, hasPass: boolean) {
		pendingRoom = roomName;
		if (hasPass) {
			showPasswordPrompt = true;
			setTimeout(() => {
				const input = document.getElementById('prompt-password') as HTMLInputElement;
				if (input) input.focus();
			}, 100);
		} else {
			joinRoom('join', roomName, '');
		}
	}

	function confirmJoinWithPassword() {
		const password = (document.getElementById('prompt-password') as HTMLInputElement)?.value || '';
		joinRoom('join', pendingRoom, password);
		showPasswordPrompt = false;
	}

	function handlePasswordKeypress(e: KeyboardEvent) {
		if (e.key === 'Enter') confirmJoinWithPassword();
	}

	function joinPrivateRoom() {
		const privateRoomInput = document.getElementById('private-room-name') as HTMLInputElement;
		const privatePasswordInput = document.getElementById(
			'private-room-password'
		) as HTMLInputElement;
		const roomName = privateRoomInput?.value?.trim();
		const roomPassword = privatePasswordInput?.value || '';

		if (!roomName) {
			alert('Please enter a room name');
			return;
		}

		joinRoom('join', roomName, roomPassword);
	}

	function joinRoom(action: string, roomNameOverride?: string, passwordOverride?: string) {
		const roomNameInput = document.getElementById('room-name') as HTMLInputElement;
		const roomPasswordInput = document.getElementById('room-password') as HTMLInputElement;
		const roomPrivateInput = document.getElementById('room-private') as HTMLInputElement;
		const roomName = roomNameOverride ?? (roomNameInput?.value?.trim() || 'default');
		const roomPassword = passwordOverride ?? (roomPasswordInput?.value || '');
		const isPrivate = roomPrivateInput?.checked ?? false;
		const username = myUsername || guestUsername;

		if (ws) ws.close();
		messages = [];
		currentRoom = roomName;
		roomUserCount = 0;

		const stored = loadMessages(roomName);
		stored.forEach((m: Message) => (messages = [...messages, m]));

		setTimeout(() => {
			const chatbox = document.getElementById('chatbox');
			if (chatbox) chatbox.scrollTop = chatbox.scrollHeight;
		}, 10);

		ws = new WebSocket(
			`${WS_URL}/ws?room=${encodeURIComponent(roomName)}&username=${encodeURIComponent(username)}&action=${action}&password=${encodeURIComponent(roomPassword)}&private=${isPrivate}`
		);
		ws.onopen = () => {
			fetchRooms();
		};
		ws.onmessage = (e) => {
			const isSys = e.data.startsWith('SYS:');
			addMessage(e.data, isSys);

			if (isSys && e.data.includes('Users in room:')) {
				const match = e.data.match(/Users in room: (\d+)/);
				if (match) {
					roomUserCount = parseInt(match[1], 10);
				}
			}

			const stored = loadMessages(roomName);
			const {
				sender,
				text: msgText,
				isMine
			} = isSys ? { sender: undefined, text: e.data, isMine: false } : parseMessage(e.data);
			stored.push({ text: msgText, isSys, sender, timestamp: new Date(), isMine });
			saveMessages(roomName, stored);
			fetchRooms();
		};
		ws.onerror = () => {
			alert('Failed to join room. Check password if required.');
			messages = [];
			currentRoom = '';
		};
	}

	function leaveRoom() {
		if (ws) {
			ws.close();
			ws = null;
		}
		messages = [];
		currentRoom = '';
		roomUserCount = 0;
	}

	function send() {
		if (messageInput && ws && ws.readyState === WebSocket.OPEN) {
			ws.send(messageInput);
			messageInput = '';
		}
	}

	function handleKeypress(e: KeyboardEvent) {
		if (e.key === 'Enter') send();
	}

	function handleLoginKeypress(e: KeyboardEvent) {
		if (e.key === 'Enter') login();
	}

	function handleRoomKeypress(e: KeyboardEvent) {
		if (e.key === 'Enter') joinRoom('create');
	}
</script>

<div class="app-header">
	<div class="header-left">
		<h1>TempChat</h1>
		<span class="header-tagline">Quick & Temporary Chat</span>
	</div>
	<div class="header-right">
		<button class="disclaimer-btn" onclick={() => (showDisclaimer = true)}>⚠️ Disclaimer</button>
	</div>
</div>

<div class="container">
	<div class="main-content">
		{#if showLogin && !myUsername && !currentRoom}
			<div id="login-screen">
				<input
					type="text"
					id="username"
					placeholder="Enter your username"
					autocomplete="off"
					onkeypress={handleLoginKeypress}
				/>
				<button onclick={login}>Login</button>
			</div>
		{/if}

		{#if myUsername}
			<div id="user-info">
				<span>Logged in as <strong>{myUsername}</strong></span>
				<button onclick={logout} class="secondary">Logout</button>
			</div>
		{/if}

		{#if showRoomControls || !currentRoom}
			<div id="room-controls">
				<input
					type="text"
					id="room-name"
					placeholder="Room name"
					autocomplete="off"
					onkeypress={handleRoomKeypress}
				/>
				<input
					type="password"
					id="room-password"
					placeholder="Password (optional)"
					autocomplete="off"
					onkeypress={handleRoomKeypress}
				/>
				<label>
					<input type="checkbox" id="room-private" />
					Private
				</label>
				<button onclick={() => joinRoom('create')}>Create Room</button>
			</div>
		{/if}

		{#if currentRoom}
			<div id="room-info">
				<span>📢 Room: <strong>{currentRoom}</strong> · 👥 {roomUserCount} users</span>
				<button onclick={leaveRoom} class="success">Leave</button>
			</div>
		{/if}

		<div id="chatbox" bind:this={chatbox}>
			{#each messages as msg}
				<div
					class="message"
					class:sys={msg.isSys}
					class:mine={msg.isMine}
					class:theirs={!msg.isSys && !msg.isMine}
				>
					{#if !msg.isSys && !msg.isMine && msg.sender}
						<div class="message-header">
							<span class="sender-name">{msg.sender}</span>
							<span class="message-time">{formatTime(msg.timestamp)}</span>
						</div>
					{/if}
					<div class="message-content">{msg.text}</div>
				</div>
			{/each}
		</div>

		<div id="input-area">
			<input
				type="text"
				id="message"
				placeholder="Type a message..."
				autocomplete="off"
				bind:value={messageInput}
				onkeypress={handleKeypress}
			/>
			<button onclick={send}>Send</button>
		</div>
	</div>

	<div class="sidebar">
		<h3>Public Rooms</h3>
		<button class="refresh-btn" onclick={fetchRooms}>↻ Refresh</button>
		<div class="room-list">
			{#each roomList as room}
				<div class="room-item">
					<div class="room-info-left">
						<span>{room.name}</span>
						<span class="room-count">({room.userCount})</span>
					</div>
					{#if room.name !== currentRoom}
						<button onclick={() => promptJoinRoom(room.name, room.hasPass)}>Join</button>
					{/if}
				</div>
			{/each}
			{#if roomList.length === 0}
				<p class="no-rooms">No rooms yet. Create one!</p>
			{/if}
		</div>

		<div class="private-join">
			<input
				type="text"
				id="private-room-name"
				placeholder="Private room name"
				autocomplete="off"
			/>
			<input
				type="password"
				id="private-room-password"
				placeholder="Password (if needed)"
				autocomplete="off"
			/>
			<button onclick={joinPrivateRoom}>Join Private</button>
		</div>
	</div>
</div>

{#if showPasswordPrompt}
	<div class="modal-overlay">
		<div class="modal">
			<h3>🔒 Enter Room Password</h3>
			<input
				type="password"
				id="prompt-password"
				placeholder="Password"
				autocomplete="off"
				onkeypress={handlePasswordKeypress}
			/>
			<div class="modal-actions">
				<button onclick={confirmJoinWithPassword}>Join</button>
				<button onclick={() => (showPasswordPrompt = false)} class="secondary">Cancel</button>
			</div>
		</div>
	</div>
{/if}

{#if showDisclaimer}
	<div class="modal-overlay">
		<div class="modal disclaimer-modal">
			<h3>⚠️ Disclaimer</h3>
			<div class="disclaimer-content">
				<p><strong>This is not a replacement for real chat applications.</strong></p>
				<p>This site is intended for <strong>quick and temporary chats only</strong>.</p>
				<hr />
				<p class="warning"><strong>Do NOT share sensitive information here.</strong></p>
				<p class="note">Use at your own risk.</p>
			</div>
			<button
				onclick={() => {
					localStorage.setItem('disclaimer_seen', 'true');
					showDisclaimer = false;
				}}>I Understand</button
			>
		</div>
	</div>
{/if}
