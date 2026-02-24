<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import './+page.css';

	interface Message {
		text: string;
		isSys: boolean;
	}

	interface Room {
		name: string;
		hasPass: boolean;
		userCount: number;
	}

	let ws: WebSocket | null = null;
	let currentRoom = '';
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
	// const WS_URL = (location.protocol === 'https:' ? 'wss:' : 'ws:') + '//' + location.host;
	// const API_URL = '';
	const WS_URL = 'wss://temp-chat-production-45a1.up.railway.app';
	const API_URL = 'https://temp-chat-production-45a1.up.railway.app';

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
		return stored ? JSON.parse(stored) : [];
	}

	function addMessage(text: string, isSys: boolean) {
		messages = [...messages, { text, isSys }];
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
			stored.push({ text: e.data, isSys });
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

<div class="mt-4 flex flex-col items-center">
	<h1 class="text-4xl">TempChat</h1>
	<button class="disclaimer-btn" onclick={() => (showDisclaimer = true)}> ⚠️ Disclaimer </button>
</div>

<div id="user-info">
	{#if myUsername}
		Logged in as {myUsername} <button onclick={logout}>Logout</button>
	{:else}
		Logged in as {guestUsername}
	{/if}
</div>

<div class="container">
	<div class="main-content">
		{#if showLogin && !myUsername && !currentRoom}
			<div id="login-screen">
				<input
					type="text"
					id="username"
					placeholder="Username"
					autocomplete="off"
					onkeypress={handleLoginKeypress}
				/>
				<button onclick={login}>Login</button>
				<button onclick={() => (showLogin = false)}>Cancel</button>
			</div>
		{/if}

		<div id="room-controls">
			<input
				type="text"
				id="room-name"
				placeholder="New room name"
				autocomplete="off"
				onkeypress={handleKeypress}
			/>
			<input
				type="password"
				id="room-password"
				placeholder="Room password (optional)"
				autocomplete="off"
				onkeypress={handleKeypress}
			/>
			<label>
				<input type="checkbox" id="room-private" />
				Private
			</label>
			<button onclick={() => joinRoom('create')}>Create Room</button>
		</div>

		{#if currentRoom}
			<div id="room-info">
				Room: {currentRoom} | Users: {roomUserCount}
				<button onclick={leaveRoom}>Leave Room</button>
			</div>
		{/if}

		<div id="chatbox">
			{#each messages as msg}
				<div class:sys={msg.isSys}>{msg.text}</div>
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
		<button onclick={fetchRooms}>Refresh</button>
		{#each roomList as room}
			<div class="room-item">
				<span>{room.name} ({room.userCount})</span>
				{#if room.name !== currentRoom}
					<button onclick={() => promptJoinRoom(room.name, room.hasPass)}>Join</button>
				{/if}
			</div>
		{/each}
		{#if roomList.length === 0}
			<p>No rooms yet</p>
		{/if}

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
			<button onclick={joinPrivateRoom}>Join Private Room</button>
		</div>
	</div>
</div>

{#if showPasswordPrompt}
	<div class="modal-overlay">
		<div class="modal">
			<h3>Enter Room Password</h3>
			<input
				type="password"
				id="prompt-password"
				placeholder="Password"
				autocomplete="off"
				onkeypress={handlePasswordKeypress}
			/>
			<button onclick={confirmJoinWithPassword}>Join</button>
			<button onclick={() => (showPasswordPrompt = false)}>Cancel</button>
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
