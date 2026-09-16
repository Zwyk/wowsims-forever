import assert from 'node:assert/strict';
import { link, mkdir, mkdtemp, rm, symlink, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import { requiredFiles, validatePagesArtifact } from './check_pages_artifact.mjs';

async function artifact(t) {
	const root = await mkdtemp(path.join(os.tmpdir(), 'wowsims-pages-test-'));
	t.after(() => rm(root, { recursive: true, force: true }));
	for (const filename of requiredFiles) await writeFile(path.join(root, filename), 'fixture');
	return root;
}

test('accepts the complete preview and nested regular assets', async t => {
	const root = await artifact(t);
	await mkdir(path.join(root, 'assets'));
	await writeFile(path.join(root, 'assets', 'sample.txt'), 'sample');
	assert.deepEqual(await validatePagesArtifact(root), { files: requiredFiles.length + 1, bytes: requiredFiles.length * 7 + 6 });
});

test('rejects missing and empty required files', async t => {
	const root = await artifact(t);
	await writeFile(path.join(root, 'index.html'), '');
	await assert.rejects(validatePagesArtifact(root), /Required Pages file is empty: index.html/);
	await rm(path.join(root, 'index.html'));
	await assert.rejects(validatePagesArtifact(root), /Required Pages file is missing: index.html/);
});

test('rejects symbolic links to files, directories, or the artifact root', async t => {
	const root = await artifact(t);
	await symlink('index.html', path.join(root, 'linked.html'));
	await assert.rejects(validatePagesArtifact(root), /without links: linked.html/);
	await rm(path.join(root, 'linked.html'));
	await symlink('.', path.join(root, 'linked-directory'));
	await assert.rejects(validatePagesArtifact(root), /without links: linked-directory/);
	await assert.rejects(validatePagesArtifact(path.join(root, 'linked-directory')), /root must be a directory/);
});

test('rejects hard-linked files', async t => {
	const root = await artifact(t);
	await link(path.join(root, 'index.html'), path.join(root, 'linked.html'));
	await assert.rejects(validatePagesArtifact(root), /without links:/);
});

test('rejects hidden files or directories and oversized artifacts', async t => {
	const root = await artifact(t);
	await writeFile(path.join(root, '.secret'), 'private');
	await assert.rejects(validatePagesArtifact(root), /Hidden content/);
	await rm(path.join(root, '.secret'));
	await mkdir(path.join(root, '.git'));
	await assert.rejects(validatePagesArtifact(root), /Hidden content/);
	await rm(path.join(root, '.git'), { recursive: true });
	await assert.rejects(validatePagesArtifact(root, { maxBytes: 1 }), /safety limit/);
});

test('rejects a file instead of an artifact directory', async t => {
	const root = await artifact(t);
	await assert.rejects(validatePagesArtifact(path.join(root, 'index.html')), /root must be a directory/);
});
