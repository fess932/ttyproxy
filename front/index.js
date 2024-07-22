import { Terminal } from '@xterm/xterm';
import { AttachAddon } from '@xterm/addon-attach';
import { FitAddon } from '@xterm/addon-fit';

const terminal = new Terminal();
const fitAddon = new FitAddon();
terminal.loadAddon(fitAddon);
terminal.open(document.getElementById('terminal'));
fitAddon.fit();

const webSocket = new WebSocket('ws://localhost:4444/pts/2');

const sendSize = () => {
  const windowSize = {high: terminal.rows, width: terminal.cols};
  const blob = new Blob([JSON.stringify(windowSize)], {type : 'application/json'});
  webSocket.send(blob);
}

webSocket.onopen = sendSize;

const resizeScreen = () => {
  fitAddon.fit();
  sendSize();
}
window.addEventListener('resize', resizeScreen, false);

const attachAddon = new AttachAddon(webSocket);
terminal.loadAddon(attachAddon);
