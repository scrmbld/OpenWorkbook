import { Terminal } from "@xterm/xterm";

var term = new Terminal
let termElement = document.getElementById('terminal')
term.open(termElement);
term.write('Hello from \x1B[1;3;31mxterm.js\x1B[0m $');
