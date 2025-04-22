const { Terminal } = require("@xterm/xterm");

const socketProtocol = window.location.protocol === 'https' ? 'wss:' : 'ws:';
const socketUrl = `${socketProtocol}//${window.location.host}/echo`;
console.log(socketUrl)
const socket = new WebSocket(socketUrl);

socket.onmessage = (e) => {
	console.log(e.data);
	term.write(e.data);
}

// we assume that xtermjs is loaded earlier in the terminal (at least I think that's what window.Terminal means)
// that's where the css needs to be loaded anyway
var term = new Terminal({
	cursorBlink: true
});
let termElement = document.getElementById('terminal');
term.open(termElement);
term.write('Hello from \x1B[1;3;31mxterm.js\x1B[0m $');

function init() {
	if (term._initialized) {
		return;
	}

	term._initialized = true

	term.onKey((keyObj) => {
		runCommand(keyObj.key);
	});

	// allow pasting from clipboard
	// <C-S-v> just like most linux terminal emulators
	term.attachCustomKeyEventHandler((e) => {
		if ((e.ctrlKey && e.shiftKey) && e.key === 'v') {
			navigator.clipboard.readText().then((text) => {
				runCommand(text);
			});
			return false;
		}
		return true;
	})

	function runCommand(command) {
		socket.send(command);
	}
}

init();
