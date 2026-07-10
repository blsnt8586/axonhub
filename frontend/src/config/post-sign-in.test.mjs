import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'post-sign-in.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { getSafeInternalRedirect } = await import(moduleUrl);

test('keeps an internal post-login route including query and hash', () => {
  assert.equal(
    getSafeInternalRedirect('/project/api-keys?status=enabled#keys', 'https://axonhub.example'),
    '/project/api-keys?status=enabled#keys'
  );
});

test('rejects protocol-relative and external post-login redirects', () => {
  assert.equal(getSafeInternalRedirect('//evil.example/path', 'https://axonhub.example'), undefined);
  assert.equal(getSafeInternalRedirect('https://evil.example/path', 'https://axonhub.example'), undefined);
});
