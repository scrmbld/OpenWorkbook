import { CodeJar } from "/js/include/codejar.js";

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
	let result = [];
	for (let i = 0; i < s.length; i += 512) {
		result.push(s.slice(i, i + 512));
	}

	return result;
}

async function highlightAsync(element) {
	Prism.highlightElement(element);
}

// start the terminals and add button event listeners
let terms = new Map();

// function to initialize terminal elements, used in the script tag in the CodeExercise templ
export function startTerm(probId) {
	const termId = `codeterminal${probId}`;
	const termElement = document.getElementById(termId);
	const newTerm = new Terminal({
		cursorBlink: true
	});
	newTerm.open(termElement);
	newTerm._initialized = true;
	terms.set(probId, newTerm);

	newTerm.write("Click 'Run' to run your code\r\n")
}

export function startCodeJar(probId) {
	const jarElement = document.getElementById(`codearea${probId}`);
	CodeJar(jarElement, highlightAsync);
}

// sends our code to the server to run and connects to the instance that's created
export function runCode(e) {
	const probId = e.target.id.replace("coderun", "");
	const codeText = document.getElementById("codearea" + probId).textContent;
	console.log(codeText);
	const term = terms.get(probId);

	const socketProtocol = window.location.protocol === 'https' ? 'wss:' : 'ws:';
	const socketUrl = `${socketProtocol}//${window.location.host}/echo`;
	const socket = new WebSocket(socketUrl)

	socket.onclose = (e) => {
		console.log(`closed: ${e.code}`);
		deactivateTerm();
	};

	socket.onopen = () => {

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
		try {
			sendProcMsg(socket, msg);
		} catch (err) {
			console.log(`error sending code EOF: ${err.message}`);
		}

		socket.onmessage = (e) => {
			console.log(e.data);
			msg = JSON.parse(e.data);
			// NOTE: this could get expensive
			term.write(msg.body.replace(/\n/g, "\n\r"));
		}

		function activateTerm() {
			term.clear();

			const newTheme = { background: "#000000" };
			term.options.theme = { ...newTheme };

			term.attachCustomKeyEventHandler((e) => {
				if (socket.readyState == socket.OPEN) {
					// make <C-d> send EOF
					if (e.ctrlKey && e.key === 'd') {
						msg = new ProcMessage("EOF", "stdin");
						try {
							sendProcMsg(socket, msg);
						} catch (err) {
							console.log(`error seding stdin EOF: ${err.message}`);
						}
						return false;
					}

				}

				return true;
			});

			term.onKey((keyObj) => {
				if (socket.readyState == socket.OPEN) {

					let keyStr = keyObj.key.replace(/\r/g, "\n\r");
					term.write(keyStr);
					msg = new ProcMessage("stdin", keyStr);
					try {
						sendProcMsg(socket, msg);
					} catch (err) {
						console.log(err);
						return;
					}
				}
			});
		}

		activateTerm();
	}

	function deactivateTerm() {
		term.write("Done!\r\n");
		term.blur();
	}
}
