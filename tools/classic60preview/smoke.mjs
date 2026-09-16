import assert from 'node:assert/strict';
import { webcrypto } from 'node:crypto';
import { readFile, readdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = fileURLToPath(new URL('../../', import.meta.url));
const artifact = resolve(root, process.argv[2] || 'dist/forever-preview');
assert.deepEqual((await readdir(artifact)).sort(), ['app.js', 'build-info.json', 'classic60.wasm', 'index.html', 'style.css', 'wasm_exec.js', 'baseline.html', 'forever.js', 'forever.css', 'forever-worker.js', 'logo.png', 'paladin.jpg'].sort());
const build = JSON.parse(await readFile(resolve(artifact, 'build-info.json'), 'utf8'));
assert.match(build.commit, /^[a-f\d]{40}$/i);
assert.equal(typeof build.dirty, 'boolean');
globalThis.crypto ??= webcrypto;
vm.runInThisContext(await readFile(resolve(artifact, 'wasm_exec.js'), 'utf8'), { filename: 'wasm_exec.js' });
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(await readFile(resolve(artifact, 'classic60.wasm')), go.importObject);
void go.run(instance).then(() => { throw new Error('Preview engine unexpectedly exited.'); });
assert.equal(typeof globalThis.classic60Preview, 'function');

assert.equal(typeof globalThis.foreverRet, 'function');
const ret = JSON.parse(globalThis.foreverRet(JSON.stringify({ iterations: 3, duration: 60 })));
assert.equal(ret.error, undefined, ret.error);
assert.ok(ret.dps > 0);
assert.ok(ret.actions.some(action => action.name === 'Holy Strike' && action.dps > 0));
assert.ok(ret.actions.some(action => action.name === 'Hammer of Wrath' && action.dps > 0));
assert.equal(globalThis.foreverRet(JSON.stringify({ iterations: 3, duration: 60 })), JSON.stringify(ret));
console.log(`Forever Ret WASM: ${ret.dps.toFixed(2)} DPS, deterministic results.`);

function call(request) {
	const envelope = JSON.parse(globalThis.classic60Preview(JSON.stringify(request)));
	assert.equal(envelope.error, undefined, envelope.error);
	assert.ok(Object.hasOwn(envelope, 'data'));
	return envelope.data;
}

function checkFinite(value) {
	if (typeof value === 'number') assert.ok(Number.isFinite(value), `Non-finite number: ${value}`);
	else if (value && typeof value === 'object') for (const item of Object.values(value)) checkFinite(item);
}

function stat(data, name) {
	const normalize = value => value.toLowerCase().replace(/[^a-z]/g, '');
	const found = data.stats.find(entry => normalize(entry.name) === normalize(name));
	assert.ok(found, `Missing stat ${name}`);
	return found.value;
}

function close(actual, expected) {
	assert.ok(Math.abs(actual - expected) < 1e-9, `Expected ${expected}; got ${actual}`);
}

const catalog = call({ operation: 'catalog' });
assert.equal(catalog.length, 40);
assert.equal(new Set(catalog.map(entry => `${entry.race}/${entry.class}`)).size, 40);
for (const entry of catalog) {
	assert.equal(typeof entry.raceName, 'string');
	assert.equal(typeof entry.className, 'string');
	for (const passiveRacials of [false, true]) {
		const data = call({ operation: 'calculate', level: 60, race: entry.race, class: entry.class, passiveRacials });
		checkFinite(data);
		assert.equal(data.level, 60);
		assert.equal(data.race, entry.race);
		assert.equal(data.class, entry.class);
		assert.equal(data.passiveRacials, passiveRacials);
		assert.ok(data.stats.length > 0);
		assert.equal(data.avoidance.canBlock, false);
		assert.ok(data.armorDamageMultiplierAgainst63 > 0 && data.armorDamageMultiplierAgainst63 <= 1);
	}
}
const gnomeMage = catalog.find(entry => entry.raceName === 'Gnome' && entry.className === 'Mage');
assert.ok(gnomeMage);
const calculateMage = passiveRacials => call({ operation: 'calculate', level: 60, race: gnomeMage.race, class: gnomeMage.class, passiveRacials });
const racialMage = calculateMage(true);
close(stat(racialMage, 'Intellect'), 134.4);
close(stat(racialMage, 'Mana'), 2949);
close(stat(racialMage, 'Spell Crit'), 2.45792);
close(stat(calculateMage(false), 'Mana'), 2853);
for (const request of [
	{ operation: 'unknown' },
	{ operation: 'calculate', level: 70, race: gnomeMage.race, class: gnomeMage.class },
	{ operation: 'calculate', level: 60, race: -1, class: gnomeMage.class },
	{ operation: 'calculate', level: 60, race: gnomeMage.race, class: 999 },
]) {
	const envelope = JSON.parse(globalThis.classic60Preview(JSON.stringify(request)));
	assert.equal(typeof envelope.error, 'string');
	assert.ok(envelope.error.length > 0);
	assert.equal(envelope.data, undefined);
}
const invalidJSON = JSON.parse(globalThis.classic60Preview('{'));
assert.equal(typeof invalidJSON.error, 'string');
for (const args of [[], [42], ['{"operation":"catalog"} {}'], ['{"operation":"catalog","unexpected":true}']]) {
	const envelope = JSON.parse(globalThis.classic60Preview(...args));
	assert.equal(typeof envelope.error, 'string');
	assert.ok(envelope.error.length > 0);
	assert.equal(envelope.data, undefined);
}
console.log('Classic 60 WASM smoke checks passed: 40 race/class pairs, 80 baselines, literal racial stats and invalid-input rejection.');
process.exit(0);
