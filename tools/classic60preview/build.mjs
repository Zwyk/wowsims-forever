import { execFileSync } from 'node:child_process';
import { copyFile, mkdir, readdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../../', import.meta.url));
const output = join(root, 'dist', 'forever-preview');
const go = process.env.GO || 'go';
const expectedFiles = new Set(['index.html', 'app.js', 'style.css', 'wasm_exec.js', 'classic60.wasm', 'build-info.json', 'baseline.html', 'forever.js', 'forever.css', 'forever-worker.js', 'logo.png', 'paladin.jpg']);
const run = (command, args, options = {}) => (execFileSync(command, args, { cwd: root, encoding: 'utf8', ...options }) || '').trim();

await mkdir(output, { recursive: true });
for (const entry of await readdir(output)) {
	if (!expectedFiles.has(entry)) throw new Error(`Unexpected file in preview output: ${entry}. Use a clean dedicated build directory.`);
}

const commit = run('git', ['rev-parse', 'HEAD']);
if (!/^[a-f\d]{40}$/i.test(commit)) throw new Error('Expected a full Git commit SHA for build provenance.');
if (process.env.GITHUB_SHA && process.env.GITHUB_SHA !== commit) throw new Error('GITHUB_SHA does not match the checked-out commit.');
const dirty = run('git', ['status', '--porcelain', '--untracked-files=normal']) !== '';
const goroot = run(go, ['env', 'GOROOT']);
run(go, ['build', '-ldflags', '-w -s', '-o', join(output, 'classic60.wasm'), './cmd/classic60preview'], {
	env: { ...process.env, GOOS: 'js', GOARCH: 'wasm', CGO_ENABLED: '0' },
	stdio: ['ignore', 'inherit', 'inherit'],
});
await Promise.all([
	...['app.js', 'style.css'].map(file => copyFile(join(root, 'ui', 'classic60preview', file), join(output, file))),
	copyFile(join(root, 'ui', 'classic60preview', 'index.html'), join(output, 'baseline.html')),
	...['index.html', 'forever.js', 'forever.css', 'forever-worker.js'].map(file => copyFile(join(root, 'ui', 'forever', file), join(output, file))),
	copyFile(join(root, 'assets', 'img', 'WoW-Simulator-Icon.png'), join(output, 'logo.png')),
	copyFile(join(root, 'assets', 'img', 'retribution_paladin.jpg'), join(output, 'paladin.jpg')),
	copyFile(join(goroot, 'lib', 'wasm', 'wasm_exec.js'), join(output, 'wasm_exec.js')),
]);
await writeFile(join(output, 'build-info.json'), `${JSON.stringify({ commit, dirty, builtAt: new Date().toISOString() }, null, 2)}\n`);
console.log(`Built Forever Ret simulator and Classic 60 baseline preview in ${output}`);
