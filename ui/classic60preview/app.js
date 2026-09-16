'use strict';

(() => {
	const byId = id => document.getElementById(id);
	const raceSelect = byId('race');
	const classSelect = byId('class');
	const racials = byId('passive-racials');
	const controls = byId('character-controls');
	const results = byId('results');
	const status = byId('status');
	const error = byId('error');
	const download = byId('download');
	const format = new Intl.NumberFormat('en-US', { maximumFractionDigits: 6 });
	let catalog = [];
	let report = null;
	let buildInfo = null;
	let failed = false;

	function fail(message, fatal = false) {
		report = null;
		results.hidden = true;
		download.disabled = true;
		status.textContent = '';
		error.textContent = message;
		error.hidden = false;
		if (fatal) {
			failed = true;
			controls.disabled = true;
		}
	}

	function call(request) {
		const response = JSON.parse(globalThis.classic60Preview(JSON.stringify(request)));
		if (response.error) throw new Error(response.error);
		if (!Object.hasOwn(response, 'data')) throw new Error('The engine returned no result.');
		return response.data;
	}

	function addOption(select, value, name) {
		const option = document.createElement('option');
		option.value = String(value);
		option.textContent = name;
		select.append(option);
	}

	function updateClasses() {
		const previous = classSelect.value;
		classSelect.replaceChildren();
		for (const entry of catalog.filter(entry => entry.race === Number(raceSelect.value))) {
			addOption(classSelect, entry.class, entry.className);
		}
		if ([...classSelect.options].some(option => option.value === previous)) classSelect.value = previous;
	}

	function finite(value) {
		if (typeof value !== 'number' || !Number.isFinite(value)) throw new Error('The engine returned an invalid numeric result.');
		return format.format(value);
	}

	function sourceLink(target, prefix, hash, label) {
		target.replaceChildren();
		target.append(document.createTextNode(prefix));
		if (typeof hash === 'string' && /^[a-f\d]{40}$/i.test(hash)) {
			const link = document.createElement('a');
			link.href = `https://github.com/wowsims/classic/commit/${hash}`;
			link.textContent = `${label} ${hash.slice(0, 12)}`;
			target.append(link);
		} else {
			target.append(document.createTextNode('Source revision unavailable.'));
		}
	}

	function render(data) {
		const cards = document.createDocumentFragment();
		for (const stat of data.stats) {
			const card = document.createElement('div');
			card.className = 'stat';
			const name = document.createElement('dt');
			name.textContent = stat.name;
			const value = document.createElement('dd');
			value.textContent = `${finite(stat.value)}${stat.unit === '%' ? '%' : stat.unit ? ` ${stat.unit}` : ''}`;
			card.append(name, value);
			cards.append(card);
		}
		byId('stats').replaceChildren(cards);
		byId('character-summary').textContent =
			`Level ${data.level} ${data.raceName} ${data.className} · ${data.passiveRacials ? 'Supported passive racials included' : 'Before passive racial effects'}`;
		byId('base-mana').textContent = data.hasMana ? finite(data.baseMana) : 'Not a mana user';
		byId('armor-multiplier').textContent = `${finite(data.armorDamageMultiplierAgainst63 * 100)}% (${finite(data.armorDamageMultiplierAgainst63)}×)`;
		const rows = document.createDocumentFragment();
		for (const [name, chance, capability] of [
			['Dodge', data.avoidance.dodgePercent, 'Available'],
			['Parry', data.avoidance.parryPercent, data.avoidance.canParry ? 'Available' : 'Unavailable · effective 0%'],
			['Block', data.avoidance.blockPercent, data.avoidance.canBlock ? 'Available' : 'Unavailable · effective 0%'],
		]) {
			const row = document.createElement('tr');
			const heading = document.createElement('th');
			heading.scope = 'row';
			heading.textContent = name;
			const percentage = document.createElement('td');
			percentage.textContent = `${finite(chance)}%`;
			const available = document.createElement('td');
			available.textContent = capability;
			row.append(heading, percentage, available);
			rows.append(row);
		}
		byId('avoidance').replaceChildren(rows);
		byId('rounding').textContent = `Rounding policy: ${data.rounding}`;
		sourceLink(byId('source'), 'Classic reference: ', data.sourceCommit, 'wowsims/classic');
	}

	function calculate() {
		if (failed) return;
		try {
			const request = {
				operation: 'calculate',
				level: 60,
				race: Number(raceSelect.value),
				class: Number(classSelect.value),
				passiveRacials: racials.checked,
			};
			const data = call(request);
			render(data);
			report = { request, data };
			error.hidden = true;
			results.hidden = false;
			download.disabled = false;
			status.textContent = `${data.raceName} ${data.className} baseline ready, ${data.passiveRacials ? 'with' : 'without'} supported passive racials. Calculated locally in your browser.`;
		} catch (cause) {
			fail(`Could not calculate this baseline: ${cause.message}`);
		}
	}

	async function loadBuildInfo() {
		try {
			const response = await fetch('./build-info.json', { cache: 'no-cache', signal: AbortSignal.timeout(15000) });
			if (!response.ok) throw new Error('Build information unavailable');
			const value = await response.json();
			if (typeof value.commit !== 'string' || !/^[a-f\d]{40}$/i.test(value.commit)) throw new Error('Invalid build revision');
			buildInfo = value;
			const link = document.createElement('a');
			link.href = `https://github.com/Zwyk/wowsims-forever/commit/${value.commit}`;
			link.textContent = value.commit.slice(0, 12);
			byId('build').replaceChildren(document.createTextNode('Build '), link, document.createTextNode(value.dirty ? ' · local changes included' : ''));
		} catch {
			byId('build').textContent = 'Build revision unavailable. Include this detail when reporting an issue.';
		}
	}

	async function start() {
		void loadBuildInfo();
		try {
			if (typeof globalThis.Go !== 'function' || typeof WebAssembly !== 'object')
				throw new Error('This browser could not load the Go WebAssembly runtime.');
			const go = new globalThis.Go();
			const response = await fetch('./classic60.wasm', { signal: AbortSignal.timeout(30000) });
			if (!response.ok) throw new Error(`Engine download failed (${response.status}).`);
			const { instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
			void go.run(instance).then(
				() => fail('The preview engine stopped. Reload this page to try again.', true),
				cause => fail(`The preview engine stopped: ${cause.message}`, true),
			);
			if (typeof globalThis.classic60Preview !== 'function') throw new Error('The preview engine did not register its interface.');
			catalog = call({ operation: 'catalog' });
			if (!Array.isArray(catalog) || catalog.length !== 40) throw new Error('The supported character catalog could not be loaded.');
			const races = new Map(catalog.map(entry => [entry.race, entry.raceName]));
			for (const [race, name] of races) addOption(raceSelect, race, name);
			updateClasses();
			controls.disabled = false;
			calculate();
		} catch (cause) {
			fail(`Preview unavailable: ${cause.message} Reload the page to retry.`, true);
		}
	}

	byId('character-form').addEventListener('submit', event => event.preventDefault());
	raceSelect.addEventListener('change', () => {
		updateClasses();
		calculate();
	});
	classSelect.addEventListener('change', calculate);
	racials.addEventListener('change', calculate);
	download.addEventListener('click', () => {
		if (!report) return;
		const payload = {
			...report,
			build: buildInfo,
			generatedAt: new Date().toISOString(),
			scope: 'Experimental Classic source-code baseline; not a DPS simulation or verified Forever mechanics.',
		};
		const blob = new Blob([`${JSON.stringify(payload, null, 2)}\n`], { type: 'application/json' });
		const url = URL.createObjectURL(blob);
		const anchor = document.createElement('a');
		anchor.href = url;
		anchor.download = `classic60-${report.data.race}-${report.data.class}-${report.data.passiveRacials ? 'racials' : 'pre-racial'}.json`;
		document.body.append(anchor);
		anchor.click();
		anchor.remove();
		setTimeout(() => URL.revokeObjectURL(url), 1000);
	});
	void start();
})();
