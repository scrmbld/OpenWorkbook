const { Terminal } = require("@xterm/xterm");

class ProcMessage {
	constructor(category, body) {
		this.category = category;
		this.body = body;
	}
}

function sendProcMsg(ws, msg) {
	console.log(msg);
	ws.send(JSON.stringify(msg));
}

function splitByIndex(s) {
	result = [];
	for (let i = 0; i < s.length; i += 512) {
		result.push(s.slice(i, i + 512));
	}

	return result;
}

// start the terminal
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

}

init();

// sends our code to the server to run and connects to the instance that's created
function runCode() {
	const codeText = document.getElementById("code-area").value;

	const socketProtocol = window.location.protocol === 'https' ? 'wss:' : 'ws:';
	const socketUrl = `${socketProtocol}//${window.location.host}/echo`;
	const socket = new WebSocket(socketUrl)

	socket.addEventListener("close", (e) => {
		console.log("closed");
		deactivateTerm();
	});

	// send the code to the server before handing things over to the terminal
	const codeSections = splitByIndex(codeText);
	for (const s of codeSections) {
		try {
			let msg = new ProcMessage("code", s);
			sendProcMsg(socket, msg);
		} catch (err) {
			console.log(`error sending code: ${err.message}`);
			return;
		}
	}
	let msg = new ProcMessage("EOF", "program");
	sendProcMsg(socket, msg);

	socket.onmessage = (e) => {
		console.log(e.data);
		msg = JSON.parse(e.data);
		// NOTE: this could get expensive
		term.write(msg.body.replace(/\n/g, "\n\r"));
	}

	function activateTerm() {
		term.onKey((keyObj) => {
			// make <C-d> send EOF
			if (keyObj.ctrlKey && keyObj.key === 'd') {
				msg = new ProcMessage("EOF", "stdin")
				try {
					sendProcMsg(socket, msg);
				} catch (err) {
					console.log(err.message);
					return;
				}
				return;
			}

			keyStr = keyObj.key.replace(/\r/g, "\n\r");
			term.write(keyStr);
			msg = new ProcMessage("stdin", keyStr);
			try {
				sendProcMsg(socket, msg);
			} catch (err) {
				console.log(err);
				return;
			}
		});
	}

	activateTerm();

	function deactivateTerm() {
		term.onKey(undefined);
	}
}

const runBtn = document.getElementById("run-button");
runBtn.addEventListener("click", runCode);
