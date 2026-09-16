import { lstat, readdir } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

export const requiredFiles = ['index.html', 'app.js', 'style.css', 'wasm_exec.js', 'classic60.wasm', 'build-info.json'];

// Stay below Pages' supported 1 GB site limit, including packaging overhead.
const maxArtifactBytes = 900 * 1024 * 1024;

export async function validatePagesArtifact(root, { maxBytes = maxArtifactBytes } = {}) {
	if (!(await lstat(root)).isDirectory()) {
		throw new Error('Pages artifact root must be a directory, not a file or symbolic link');
	}

	let bytes = 0;
	let files = 0;
	const found = new Set();
	async function visit(directory, prefix = '') {
		for (const entry of await readdir(directory, { withFileTypes: true })) {
			const relative = prefix + entry.name;
			if (entry.name.startsWith('.')) {
				throw new Error(`Hidden content is not permitted in the Pages artifact: ${relative}`);
			}
			const filename = path.join(directory, entry.name);
			const info = await lstat(filename);
			if (info.isDirectory()) {
				await visit(filename, `${relative}/`);
			} else if (info.isFile() && info.nlink === 1) {
				if (requiredFiles.includes(relative) && info.size === 0) {
					throw new Error(`Required Pages file is empty: ${relative}`);
				}
				files++;
				bytes += info.size;
				found.add(relative);
				if (bytes > maxBytes) {
					throw new Error(`Pages artifact exceeds its ${maxBytes}-byte safety limit`);
				}
			} else {
				throw new Error(`Pages artifact must contain only regular files and directories, without links: ${relative}`);
			}
		}
	}
	await visit(root);
	for (const required of requiredFiles) {
		if (!found.has(required)) {
			throw new Error(`Required Pages file is missing: ${required}`);
		}
	}
	return { files, bytes };
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
	try {
		if (process.argv.length !== 3) throw new Error('Usage: node tools/ci/check_pages_artifact.mjs <artifact-directory>');
		const { files, bytes } = await validatePagesArtifact(process.argv[2]);
		console.log(`Pages artifact validated: ${files} regular files, ${bytes} bytes`);
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}
