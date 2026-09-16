/* Run the actual Go combat engine off the UI thread. Termination cancels it. */
importScripts('./wasm_exec.js');
const ready = (async () => {
	const go = new Go();
	const response = await fetch('./classic60.wasm');
	if (!response.ok) throw new Error(`Engine download failed (${response.status}).`);
	const { instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
	void go.run(instance);
	if (typeof self.foreverRet !== 'function') throw new Error('Ret engine is unavailable.');
})();
self.onmessage = async ({ data }) => {
	try {
		await ready;
		const result = JSON.parse(self.foreverRet(JSON.stringify(data.request)));
		self.postMessage({ id: data.id, result });
	} catch (error) {
		self.postMessage({ id: data.id, result: { error: error.message } });
	}
};
