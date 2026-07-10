import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const sourcePath = join(import.meta.dirname, 'graphql.ts');
const source = readFileSync(sourcePath, 'utf8');
let authResetCount = 0;
let assignedLocation = '';
globalThis.window = {
  location: {
    assign(path) {
      assignedLocation = path;
    },
  },
};
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText
  .replaceAll("import { toast } from 'sonner';", 'const toast = { error() {} };')
  .replaceAll(
    "import { getTokenFromStorage, useAuthStore } from '@/stores/authStore';",
    'const getTokenFromStorage = () => ""; const useAuthStore = { getState: () => ({ auth: { reset: () => { globalThis.__authResetCount += 1; } } }) };'
  )
  .replaceAll("import i18n from '@/lib/i18n';", 'const i18n = { t: (key) => key };');

globalThis.__authResetCount = 0;
const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { isUnauthorizedGraphQLError, resetExpiredAuthSession } = await import(moduleUrl);

test('does not classify upstream unauthorized provider failures as login expiration', () => {
  const error = {
    message: 'model fetch returned error: failed to fetch models: GET - https://provider.example/v1/models with status 401 Unauthorized',
    path: ['syncChannelModels'],
  };

  assert.equal(isUnauthorizedGraphQLError(error), false);
});

test('classifies explicit GraphQL authentication codes as login expiration', () => {
  assert.equal(isUnauthorizedGraphQLError({ extensions: { code: 'UNAUTHENTICATED' } }), true);
});

test('resets the complete auth session before redirecting expired users', () => {
  resetExpiredAuthSession();
  authResetCount = globalThis.__authResetCount;

  assert.equal(authResetCount, 1);
  assert.equal(assignedLocation, '/sign-in');
});
