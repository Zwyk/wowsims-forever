// Explicit, reviewed derivative of the vendored CC BY 4.0 TalentsForever snapshot.
import { readFile, writeFile } from 'node:fs/promises';
import assert from 'node:assert/strict';
const source = JSON.parse(await readFile(new URL('../../third_party/talentsforever/data.json', import.meta.url), 'utf8'));
const output = new URL('../../sim/core/forever_ret_catalog.json', import.meta.url);
const catalog = source.talents.Paladin.trees.flatMap((tree, index) => tree.talents.map(talent => ({
 ...Object.fromEntries(['name', 'max', 'row', 'col', 'req', 'desc', 'confirmed', 'complete'].map(key => [key, talent[key] ?? null])),
 tree: index,
 supported: index === 2 || talent.row < 5,
})));
const text = JSON.stringify(catalog, null, 2) + '\n';
if (process.argv.includes('--check')) assert.deepEqual(JSON.parse(await readFile(output, 'utf8')), catalog, 'Ret catalog must match the reviewed snapshot.');
else await writeFile(output, text);
