'use strict';
const $ = id => document.getElementById(id);
const storageKey = 'wowsims-forever-ret-v1';
let catalog,
	config,
	worker,
	pending = new Map(),
	requestID = 0,
	busy = false,
	runGeneration = 0,
	lastResult;
const controls = new Map();
const abilityNames = {
	hammer: 'Hammer of Wrath',
	holyStrike: 'Holy Strike',
	judgement: 'Judgement',
	exorcism: 'Exorcism',
	consecration: 'Consecration',
	holyWrath: 'Holy Wrath',
	swiftJudgement: 'Swift Judgement',
};
let priorityOrder = [];
function node(tag, text, className) {
	const el = document.createElement(tag);
	if (text !== undefined) el.textContent = text;
	if (className) el.className = className;
	return el;
}
function status(message, error = false) {
	$('status').textContent = message;
	$('status').classList.toggle('error', error);
}
function startWorker() {
	worker?.terminate();
	for (const reject of pending.values()) reject({ error: 'Simulation cancelled.' });
	pending = new Map();
	worker = new Worker('./forever-worker.js');
	worker.onmessage = ({ data }) => {
		const resolve = pending.get(data.id);
		if (resolve) {
			pending.delete(data.id);
			resolve(data.result);
		}
	};
	worker.onerror = event => {
		for (const resolve of pending.values()) resolve({ error: event.message || 'Engine failed to start. Reload to retry.' });
		pending.clear();
	};
}
function call(request) {
	return new Promise(resolve => {
		const id = ++requestID;
		pending.set(id, resolve);
		worker.postMessage({ id, request });
	});
}
function activateTab(id) {
	for (const tab of document.querySelectorAll('[role=tab]')) {
		const active = tab.getAttribute('aria-controls') === id;
		tab.setAttribute('aria-selected', String(active));
		tab.tabIndex = active ? 0 : -1;
		$(tab.getAttribute('aria-controls')).hidden = !active;
	}
}
for (const tab of document.querySelectorAll('[role=tab]')) {
	tab.onclick = () => activateTab(tab.getAttribute('aria-controls'));
	tab.onkeydown = event => {
		const tabs = [...document.querySelectorAll('[role=tab]')];
		let i = tabs.indexOf(tab);
		if (event.key === 'ArrowRight') i++;
		else if (event.key === 'ArrowLeft') i--;
		else if (event.key === 'Home') i = 0;
		else if (event.key === 'End') i = tabs.length - 1;
		else return;
		event.preventDefault();
		const next = tabs[(i + tabs.length) % tabs.length];
		next.click();
		next.focus();
	};
}
activateTab('character');
function inputField(parent, key, title, min, max, step = 1, options) {
	const label = node('label', undefined, 'field');
	label.htmlFor = `input-${key}`;
	label.append(node('span', title));
	const input = document.createElement(options ? 'select' : 'input');
	input.id = `input-${key}`;
	input.name = key;
	if (options)
		for (const [value, text] of options) {
			const option = node('option', text);
			option.value = value;
			input.append(option);
		}
	else {
		input.type = 'number';
		input.min = min;
		input.max = max;
		input.step = step;
		input.required = true;
	}
	input.value = config[key];
	input.onchange = changed;
	controls.set(key, input);
	label.append(input);
	$(parent).append(label);
}
function checkbox(parent, key, title) {
	const label = node('label', undefined, 'field check');
	const input = document.createElement('input');
	input.type = 'checkbox';
	input.id = `input-${key}`;
	input.checked = config[key];
	input.onchange = changed;
	controls.set(key, input);
	label.append(input, node('span', title));
	$(parent).append(label);
}
function readConfig() {
	for (const [key, input] of controls)
		config[key] = input.type === 'checkbox' ? input.checked : input.tagName === 'SELECT' ? input.value : input.valueAsNumber;
	config.priority = priorityOrder.filter(key => $(`priority-${key}`).checked);
	config.operation = 'simulate';
	return structuredClone(config);
}
function changed() {
	if (!catalog) return;
	readConfig();
	try {
		localStorage.setItem(storageKey, JSON.stringify(config));
	} catch {
		/* Private browsing can disable storage. */
	}
	if (lastResult) $('result-context').textContent = 'Inputs changed. Run again to update these results.';
}
function renderTalents() {
	$('talent-trees').replaceChildren();
	let total = 0;
	for (let tree = 0; tree < 3; tree++) {
		const section = node('section', undefined, 'talent-tree');
		const talents = catalog.talents.filter(t => t.tree === tree);
		const points = talents.reduce((sum, t) => sum + (config.talents[t.name] || 0), 0);
		total += points;
		section.append(node('h3', `${['Holy', 'Protection', 'Retribution'][tree]} · ${points} points`));
		const grid = node('div', undefined, 'talent-grid');
		for (const talent of talents) {
			const rank = config.talents[talent.name] || 0;
			const cell = node('div', undefined, `talent${rank ? ' spent' : ''}${talent.supported ? '' : ' unsupported'}`);
			cell.style.gridRow = talent.row;
			cell.style.gridColumn = talent.col;
			cell.append(node('span', talent.name, 'talent-name'));
			const control = node('div', undefined, 'talent-controls');
			for (const delta of [-1, 1]) {
				const button = node('button', delta === 1 ? '+' : '−');
				button.type = 'button';
				button.disabled = !talent.supported || (delta === 1 ? rank === talent.max : rank === 0);
				button.setAttribute('aria-label', `${delta === 1 ? 'Add' : 'Remove'} point in ${talent.name}`);
				button.onclick = () => {
					config.talents[talent.name] = rank + delta;
					renderTalents();
					changed();
				};
				if (delta === 1) control.append(node('output', `${rank}/${talent.max}`));
				control.append(button);
			}
			cell.append(control);
			const detail = node('details');
			detail.append(node('summary', 'Tooltip'));
			const entries = Array.isArray(talent.desc) ? talent.desc.map((text, i) => [i + 1, text]) : Object.entries(talent.desc || {});
			for (const [r, text] of entries) detail.append(node('p', `Rank ${r}: ${text}`));
			if (talent.req) detail.append(node('p', `Requires maximum rank ${talent.req}.`));
			cell.append(detail, node('span', talent.supported ? (talent.complete ? 'Full tooltip' : 'Partial evidence') : 'Outside Ret scope', 'evidence'));
			grid.append(cell);
		}
		section.append(grid);
		$('talent-trees').append(section);
	}
	$('talent-total').textContent = `${total} / 51 spent`;
	$('talent-total').classList.toggle('invalid', total > 51);
}
function renderPriority() {
	const enabled = new Set(config.priority);
	$('priority-list').replaceChildren();
	priorityOrder.forEach((key, index) => {
		const row = node('li', undefined, 'priority-row');
		const label = node('label');
		const input = document.createElement('input');
		input.type = 'checkbox';
		input.id = `priority-${key}`;
		input.checked = enabled.has(key);
		input.onchange = changed;
		label.append(input, node('span', abilityNames[key]));
		row.append(label);
		for (const direction of [-1, 1]) {
			const button = node('button', direction === -1 ? '↑' : '↓');
			button.type = 'button';
			button.disabled = index + direction < 0 || index + direction >= priorityOrder.length;
			button.setAttribute('aria-label', `Move ${abilityNames[key]} ${direction === -1 ? 'up' : 'down'}`);
			button.onclick = () => {
				readConfig();
				[priorityOrder[index], priorityOrder[index + direction]] = [priorityOrder[index + direction], priorityOrder[index]];
				renderPriority();
				changed();
			};
			row.append(button);
		}
		$('priority-list').append(row);
	});
}
function initializeForm() {
	for (const id of ['run-settings', 'stats-fields', 'weapon-fields', 'buff-fields', 'encounter-fields', 'rotation-fields', 'assumption-fields'])
		$(id).replaceChildren();
	controls.clear();
	for (const [key, title, min, max] of [
		['iterations', 'Iterations', 1, 2000],
		['duration', 'Duration (sec)', 15, 600],
		['seed', 'Random seed', 1, 2147483647],
	])
		inputField('run-settings', key, title, min, max);
	for (const [key, title, max, step] of [
		['strength', 'Strength', 3000, 1],
		['agility', 'Agility', 3000, 1],
		['intellect', 'Intellect', 3000, 1],
		['spirit', 'Spirit', 3000, 1],
		['attackPower', 'Attack power', 10000, 1],
		['spellPower', 'Spell damage', 10000, 1],
		['healingPower', 'Bonus healing → ⅓ damage', 10000, 1],
		['hit', 'Shared hit (%)', 100, 0.1],
		['crit', 'Shared crit (%)', 100, 0.1],
		['mp5', 'Mana per 5 sec', 3000, 1],
	])
		inputField('stats-fields', key, title, 0, max, step);
	for (const args of [
		['weaponMin', 'Weapon damage · minimum', 1, 2000],
		['weaponMax', 'Weapon damage · maximum', 1, 2000],
		['weaponSpeed', 'Weapon speed (sec)', 1, 5, 0.1],
		['weaponSkill', 'Weapon skill bonus', 0, 15],
		['avoidanceReduction', 'Dodge / parry reduction (%)', 0, 20, 0.1],
	])
		inputField('weapon-fields', ...args);
	inputField('buff-fields', 'blessing', 'Blessing', null, null, 1, [
		['none', 'None'],
		['might', 'Blessing of Might'],
		['wisdom', 'Blessing of Wisdom'],
		['kings', 'Blessing of Kings'],
	]);
	inputField('encounter-fields', 'targetLevel', 'Target level', 60, 63);
	inputField('encounter-fields', 'armor', 'Effective target armor', 0, 20000);
	inputField('encounter-fields', 'mobType', 'Target type', null, null, 1, [
		['humanoid', 'Humanoid'],
		['undead', 'Undead'],
		['demon', 'Demon'],
	]);
	inputField('encounter-fields', 'executePercent', 'Fight spent below 20% health (%)', 0, 100);
	inputField('rotation-fields', 'seal', 'Main seal', null, null, 1, [
		['command', 'Seal of Command'],
		['righteousness', 'Seal of Righteousness'],
	]);
	checkbox('rotation-fields', 'twist', 'Alternate seals with Twist of Light');
	checkbox('rotation-fields', 'crusaderOpener', 'Open with Judgement of the Crusader');
	inputField('rotation-fields', 'consecrationManaPercent', 'Use Consecration above mana (%)', 0, 100);
	for (const args of [
		['holyStrikeWeaponPercent', 'Holy Strike · weapon damage (%)', 1, 200, 0.1],
		['holyStrikeMin', 'Holy Strike · bonus damage minimum', 0, 2000],
		['holyStrikeMax', 'Holy Strike · bonus damage maximum', 0, 2000],
		['holyStrikeMana', 'Holy Strike · mana cost', 1, 2000],
		['vindicationPPM', 'Vindication · procs per minute', 0, 60, 0.1],
	])
		inputField('assumption-fields', ...args);
	priorityOrder = [...config.priority, ...Object.keys(abilityNames).filter(k => !config.priority.includes(k))];
	renderPriority();
	renderTalents();
}
function setBusy(value) {
	busy = value;
	$('run').disabled = value;
	$('run').textContent = value ? 'Simulating…' : 'Simulate';
	$('cancel').hidden = !value;
	$('reset').disabled = value;
	$('share').disabled = value;
	// Leave tabs and inputs usable. Results retain the exact submitted build.
}
function showResult(result) {
	lastResult = result;
	$('dps').replaceChildren(document.createTextNode(result.dps.toFixed(1) + ' '), node('small', 'DPS'));
	$('uncertainty').textContent =
		result.config.iterations > 1
			? `± ${result.ci95.toFixed(1)} DPS · approximate 95% confidence interval of the mean`
			: 'One iteration · increase iterations to estimate uncertainty';
	$('result-context').textContent =
		`${result.config.iterations.toLocaleString()} iterations · ${result.config.duration}s · level ${result.config.targetLevel} ${result.config.mobType} · seed ${result.config.seed}`;
	$('result-description').textContent =
		'Per-fight averages. Hits include normal hits, glancing hits and periodic ticks; crits are separate. Passive seal triggers do not record casts.';
	$('damage-rows').replaceChildren();
	$('damage-bars').replaceChildren();
	for (const action of result.actions) {
		const row = node('tr');
		const share = result.dps ? (action.dps / result.dps) * 100 : 0;
		for (const value of [
			action.name,
			action.dps.toFixed(1),
			`${share.toFixed(1)}%`,
			action.casts.toFixed(1),
			action.hits.toFixed(1),
			action.crits.toFixed(1),
			action.misses.toFixed(1),
			action.dodges.toFixed(1),
		])
			row.append(node('td', value));
		$('damage-rows').append(row);
		if (share > 0) {
			const bar = node('div', undefined, 'bar-row');
			const track = node('div', undefined, 'bar-track');
			const fill = node('div', undefined, 'bar-fill');
			fill.style.width = `${share}%`;
			track.append(fill);
			bar.append(node('span', action.name), track, node('span', `${share.toFixed(1)}%`));
			$('damage-bars').append(bar);
		}
	}
	$('download').disabled = false;
	activateTab('results');
	const current = readConfig();
	if (Object.keys(current).some(key => JSON.stringify(current[key]) !== JSON.stringify(result.config[key]))) {
		// Talent object key order can differ after its Go JSON round trip.
		const same = Object.keys(current).every(key =>
			key === 'talents'
				? Object.keys({ ...current.talents, ...result.config.talents }).every(
						name => (current.talents[name] || 0) === (result.config.talents[name] || 0),
					)
				: JSON.stringify(current[key]) === JSON.stringify(result.config[key]),
		);
		if (!same) $('result-context').append(node('p', 'Inputs have changed since this run.'));
	}
}
$('sim-form').noValidate = true;
$('sim-form').onsubmit = async event => {
	event.preventDefault();
	if (busy || !catalog) return;
	if (!$('sim-form').checkValidity()) {
		const invalid = $('sim-form').querySelector(':invalid');
		const panel = invalid?.closest('[role=tabpanel]');
		if (panel) activateTab(panel.id);
		$('sim-form').reportValidity();
		return;
	}
	const generation = ++runGeneration;
	const request = readConfig();
	changed();
	setBusy(true);
	status('Running the combat simulation…');
	const result = await call(request);
	if (!busy || generation !== runGeneration) return;
	setBusy(false);
	if (result.error) {
		status(result.error, true);
		return;
	}
	showResult(result);
	status('Simulation complete.');
};
$('cancel').onclick = () => {
	runGeneration++;
	setBusy(false);
	startWorker();
	status('Cancelled. Your build is ready for another run.');
};
$('reset').onclick = () => {
	config = structuredClone(catalog.defaults);
	initializeForm();
	changed();
	status('Default 18/0/33 build restored.');
};
$('share').onclick = async () => {
	const bytes = new TextEncoder().encode(JSON.stringify(readConfig()));
	const encoded = btoa(String.fromCharCode(...bytes));
	const url = new URL(location.href);
	url.hash = 'build=' + encoded;
	try {
		await navigator.clipboard.writeText(url.href);
		status('Build link copied.');
	} catch {
		history.replaceState(null, '', url);
		status('Build link added to the address bar. Copy the page URL to share it.');
	}
};
$('download').onclick = () => {
	if (!lastResult) return;
	const blob = new Blob([JSON.stringify(lastResult, null, 2)], { type: 'application/json' });
	const url = URL.createObjectURL(blob);
	const link = node('a');
	link.href = url;
	link.download = 'forever-ret-result.json';
	link.click();
	setTimeout(() => URL.revokeObjectURL(url), 1000);
};
(async () => {
	startWorker();
	catalog = await call({ operation: 'catalog' });
	if (catalog.error) {
		status(catalog.error, true);
		catalog = null;
		return;
	}
	config = structuredClone(catalog.defaults);
	let restoreMessage = '';
	try {
		let saved = localStorage.getItem(storageKey);
		if (location.hash.startsWith('#build=')) {
			const raw = location.hash.slice(7);
			if (raw.length > 90000) throw new Error('Build link is too large.');
			saved = new TextDecoder().decode(Uint8Array.from(atob(raw), c => c.charCodeAt(0)));
		}
		if (saved) {
			const parsed = JSON.parse(saved);
			if (
				!parsed ||
				typeof parsed !== 'object' ||
				Array.isArray(parsed) ||
				!Array.isArray(parsed.priority) ||
				!parsed.talents ||
				typeof parsed.talents !== 'object' ||
				Object.keys(parsed).some(key => !Object.hasOwn(config, key)) ||
				parsed.priority.some(k => !Object.hasOwn(abilityNames, k)) ||
				new Set(parsed.priority).size !== parsed.priority.length
			)
				throw new Error('Invalid saved build.');
			config = { ...config, ...parsed };
			restoreMessage = ' Build restored; its talents will be validated when you run.';
		}
	} catch {
		restoreMessage = ' Saved build could not be read; defaults restored.';
	}
	initializeForm();
	for (const note of catalog.model.notes) $('model-notes').append(node('li', note));
	setBusy(false);
	$('share').disabled = false;
	$('reset').disabled = false;
	status('Ready.' + restoreMessage);
	try {
		const response = await fetch('./build-info.json');
		if (response.ok) {
			const info = await response.json();
			$('build-version').textContent = `Build ${info.commit.slice(0, 12)}${info.dirty ? ' · local changes' : ''}`;
		}
	} catch {
		/* The simulator remains usable offline once loaded. */
	}
})();
