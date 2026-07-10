import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'workspace-selection.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { resolveSelectedWorkspaceId } = await import(moduleUrl);

const workspaces = [
  { id: 'gid://axonhub/Project/1', status: 'active' },
  { id: 'gid://axonhub/Project/2', status: 'active' },
];

test('keeps a stored active workspace membership', () => {
  assert.equal(resolveSelectedWorkspaceId(workspaces, workspaces[1].id, workspaces[0].id), workspaces[1].id);
});

test('uses the server default when storage is stale', () => {
  assert.equal(resolveSelectedWorkspaceId(workspaces, 'gid://axonhub/Project/99', workspaces[1].id), workspaces[1].id);
});

test('uses the first active workspace when no default remains', () => {
  assert.equal(resolveSelectedWorkspaceId(workspaces, null, 'gid://axonhub/Project/99'), workspaces[0].id);
});

test('clears selection after membership removal', () => {
  assert.equal(resolveSelectedWorkspaceId([], workspaces[0].id, workspaces[0].id), null);
});

test('never restores an archived workspace', () => {
  assert.equal(resolveSelectedWorkspaceId([{ id: workspaces[0].id, status: 'archived' }], workspaces[0].id, workspaces[0].id), null);
});
