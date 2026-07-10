import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'route-access.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { canAccessRoute } = await import(moduleUrl);

const emptyContext = {
  systemScopes: [],
  projectScopes: [],
  isSystemOwner: false,
  isProjectOwner: false,
};

test('project owner can open project consumer routes with an empty membership scope list', () => {
  assert.equal(
    canAccessRoute(
      { requiredScopes: ['write_requests', 'read_channels'], scopeLevel: 'project' },
      undefined,
      { ...emptyContext, isProjectOwner: true }
    ),
    true
  );
});

test('project ownership never grants system administration routes', () => {
  assert.equal(
    canAccessRoute(
      { requiredScopes: ['read_channels'], scopeLevel: 'system' },
      undefined,
      { ...emptyContext, isProjectOwner: true }
    ),
    false
  );
});

test('ordinary project member still needs a required project scope', () => {
  assert.equal(
    canAccessRoute({ requiredScopes: ['read_api_keys'], scopeLevel: 'project' }, undefined, emptyContext),
    false
  );
  assert.equal(
    canAccessRoute(
      { requiredScopes: ['read_api_keys'], scopeLevel: 'project' },
      undefined,
      { ...emptyContext, projectScopes: ['read_api_keys'] }
    ),
    true
  );
});

test('system owner keeps access to all route levels', () => {
  assert.equal(
    canAccessRoute(
      { requiredScopes: ['read_billing'], scopeLevel: 'system' },
      undefined,
      { ...emptyContext, isSystemOwner: true }
    ),
    true
  );
});
