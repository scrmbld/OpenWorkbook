const { Terminal } = require("@xterm/xterm");

const socketProtocol = window.location.protocol === 'https' ? 'wss:' : 'ws:';
const socketUrl = `${socketProtocol}//${window.location.host}/echo`;
console.log(socketUrl)
const socket = new WebSocket(socketUrl);

class ProcMessage {
	constructor(category, body) {
		this.category = category;
		this.body = body;
	}
}

socket.onmessage = (e) => {
	console.log(e.data);
	msg = JSON.parse(e.data);
	// NOTE: this could get expensive
	term.write(msg.body.replace(/\n/g, "\n\r"));
}

// we assume that xtermjs is loaded earlier in the terminal (at least I think that's what window.Terminal means)
// that's where the css needs to be loaded anyway
var term = new Terminal({
	cursorBlink: true
});
let termElement = document.getElementById('terminal');
term.open(termElement);

function init() {
	if (term._initialized) {
		return;
	}

	term._initialized = true

	term.onKey((keyObj) => {
		keyStr = keyObj.key.replace(/\r/g, "\n\r")
		term.write(keyStr);
		msg = new ProcMessage("stdin", keyStr)
		sendProcMsg(msg);
	});

	term.attachCustomKeyEventHandler((e) => {
		// allow pasting from clipboard
		// <C-S-v> just like most linux terminal emulators
		if ((e.ctrlKey && e.shiftKey) && e.key === 'v') {
			navigator.clipboard.readText().then((text) => {
				runCommand(text);
			});
			return false;
		}
		return true;
	})

	function sendProcMsg(msg) {
		socket.send(JSON.stringify(msg));
	}
}

init();
