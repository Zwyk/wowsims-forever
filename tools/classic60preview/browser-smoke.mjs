// Chrome DevTools smoke test without an extra npm dependency. CI's Ubuntu
// runner supplies Chrome; locally set CHROME_BIN to an installed binary.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, join, extname } from 'node:path';

const root = resolve(process.argv[2] || 'dist/forever-preview');
const profile = await mkdtemp(join(tmpdir(), 'forever-browser-'));
const prefix = '/wowsims-forever/';
const server = createServer(async (req, res) => {
 try {
  const path = new URL(req.url, 'http://localhost').pathname;
  if (!path.startsWith(prefix)) { res.writeHead(404); res.end(); return; }
  const file = resolve(root, path.slice(prefix.length) || 'index.html');
  if (!file.startsWith(root + '/')) throw new Error('Invalid path');
  const data = await readFile(file);
  res.setHeader('Content-Type', { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.wasm': 'application/wasm', '.json': 'application/json', '.png': 'image/png', '.jpg': 'image/jpeg' }[extname(file)] || 'application/octet-stream'); res.end(data);
 } catch { res.writeHead(404); res.end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const base = `http://127.0.0.1:${server.address().port}${prefix}`;
const chrome = spawn(process.env.CHROME_BIN || 'google-chrome', ['--headless=new', '--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu', '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank'], { stdio: ['ignore', 'ignore', 'pipe'] });
let socket;
try {
 const endpoint = await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error('Chrome startup timed out')), 30000); timer.unref();
  chrome.once('error', reject); let log = '';
  chrome.stderr.on('data', data => { log += data; const match = log.match(/DevTools listening on (ws:\/\/\S+)/); if (match) { clearTimeout(timer); resolve(match[1]); } });
 });
 socket = new WebSocket(endpoint); await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
 let id = 0; const calls = new Map(); const exceptions = [];
 socket.onmessage = ({ data }) => { const message = JSON.parse(data); if (message.method === 'Runtime.exceptionThrown') exceptions.push(message.params.exceptionDetails); const callback = calls.get(message.id); if (callback) { calls.delete(message.id); callback(message); } };
 const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
  const key = ++id; const timer = setTimeout(() => { calls.delete(key); reject(new Error(`CDP timeout: ${method}`)); }, 60000); timer.unref();
  calls.set(key, response => { clearTimeout(timer); if (response.error) reject(new Error(JSON.stringify(response.error))); else resolve(response.result); });
  socket.send(JSON.stringify({ id: key, method, params, sessionId }));
 });
 const { targetId } = await send('Target.createTarget', { url: 'about:blank' });
 const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
 const command = (method, params) => send(method, params, sessionId);
 const evaluate = async expression => { const result = await command('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }); if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails)); return result.result.value; };
 const waitFor = expression => evaluate(`(async()=>{for(let i=0;i<400;i++){if(${expression})return true;await new Promise(r=>setTimeout(r,100));}throw new Error('Timed out waiting for browser state');})()`);
 await command('Runtime.enable'); await command('Page.enable');
 await command('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
 await command('Page.navigate', { url: base });
 await waitFor(`document.querySelector('#run') && !document.querySelector('#run').disabled`);
 assert.match(await evaluate(`document.querySelector('#talent-total').textContent`), /51 \/ 51/);
 await evaluate(`document.querySelector('#input-iterations').value=3;document.querySelector('#input-duration').value=60;document.querySelector('#sim-form').requestSubmit()`);
 await waitFor(`document.querySelector('#status').textContent==='Simulation complete.'`);
 const dps = await evaluate(`parseFloat(document.querySelector('#dps').textContent)`); assert.ok(dps > 0);
 assert.match(await evaluate(`document.querySelector('#damage-rows').textContent`), /Holy Strike/);
 assert.match(await evaluate(`document.querySelector('#damage-rows').textContent`), /Hammer of Wrath/);
 assert.doesNotMatch(await evaluate(`document.querySelector('#result-context').textContent`), /changed/);
 await evaluate(`document.querySelector('#tab-character').click();document.querySelector('#input-weaponMin').value=400;document.querySelector('#input-weaponMax').value=600;document.querySelector('#sim-form').requestSubmit()`);
 await waitFor(`document.querySelector('#status').textContent==='Simulation complete.'`);
 assert.ok(await evaluate(`parseFloat(document.querySelector('#dps').textContent)`) > dps);
 // Invalid trees must produce a visible error without crashing the worker.
 await evaluate(`document.querySelector('#tab-talents').click();document.querySelector('[aria-label="Add point in Deflection"]').click();document.querySelector('#sim-form').requestSubmit()`);
 await waitFor(`document.querySelector('#status').classList.contains('error')`);
 assert.match(await evaluate(`document.querySelector('#status').textContent`), /51/);
 await evaluate(`document.querySelector('#reset').click();document.querySelector('#input-iterations').value=2000;document.querySelector('#input-duration').value=600;document.querySelector('#sim-form').requestSubmit();document.querySelector('#cancel').click()`);
 assert.match(await evaluate(`document.querySelector('#status').textContent`), /Cancelled/);
 await evaluate(`document.querySelector('#input-iterations').value=1;document.querySelector('#input-duration').value=30;document.querySelector('#sim-form').requestSubmit()`);
 await waitFor(`document.querySelector('#status').textContent==='Simulation complete.'`);
 // Saved build round-trip and usable narrow layout, including dense talent rows.
 await command('Page.reload'); await waitFor(`document.querySelector('#run') && !document.querySelector('#run').disabled`);
 assert.equal(await evaluate(`document.querySelector('#input-duration').value`), '30');
 await command('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: true });
 await evaluate(`document.querySelector('#tab-talents').click()`);
 assert.equal(await evaluate(`document.documentElement.scrollWidth <= window.innerWidth + 1`), true);
 assert.deepEqual(exceptions, []);
 console.log(`Browser smoke passed: ${dps} DPS; inputs, validation, cancellation, recovery, saved build and mobile layout.`);
} finally {
 socket?.close(); chrome.kill('SIGKILL'); server.closeAllConnections(); server.close();
 await new Promise(resolve => chrome.exitCode !== null ? resolve() : chrome.once('exit', resolve));
 await rm(profile, { recursive: true, force: true });
}
