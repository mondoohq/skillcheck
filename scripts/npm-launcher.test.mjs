// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Unit tests for how the npm launcher (npm/bin/skillcheck.js) picks the
// platform binary. The platform, the CPU arch and module resolution are
// injected, so every OS/arch combination is covered from any host without
// installing anything. The exit-code and missing-package behavior of the real
// launcher is covered by scripts/npm-package.test.mjs, which spawns it.
//
// Run: node --test scripts/npm-launcher.test.mjs

import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import path from 'node:path';
import test from 'node:test';

const require = createRequire(import.meta.url);
const { candidateKeys, resolveBinary } = require('../npm/bin/skillcheck.js');

// installed models node_modules: resolve() succeeds only for the listed
// platform packages, the way require.resolve would after npm installed them.
function installed(...pkgs) {
  const present = new Set(pkgs);
  return (request) => {
    const pkg = request.replace(/\/package\.json$/, '');
    if (!present.has(pkg)) {
      const err = new Error(`Cannot find module '${request}'`);
      err.code = 'MODULE_NOT_FOUND';
      throw err;
    }
    return path.join('/nm', pkg, 'package.json');
  };
}

const WIN_ARM = '@mondoohq/skillcheck_windows_arm64';
const WIN_X64 = '@mondoohq/skillcheck_windows_amd64';
const MAC_ARM = '@mondoohq/skillcheck_darwin_arm64';
const MAC_X64 = '@mondoohq/skillcheck_darwin_amd64';
const LINUX_ARM = '@mondoohq/skillcheck_linux_arm64';
const LINUX_X64 = '@mondoohq/skillcheck_linux_amd64';
const ALL = [WIN_ARM, WIN_X64, MAC_ARM, MAC_X64, LINUX_ARM, LINUX_X64];

test('candidateKeys: fallback order per platform', () => {
  assert.deepEqual(candidateKeys('win32', 'arm64'), ['win32_arm64', 'win32_x64']);
  assert.deepEqual(candidateKeys('win32', 'x64'), ['win32_x64', 'win32_arm64']);
  assert.deepEqual(candidateKeys('darwin', 'arm64'), ['darwin_arm64', 'darwin_x64']);
  // An x64 Node may be on an Intel Mac, which cannot run arm64.
  assert.deepEqual(candidateKeys('darwin', 'x64'), ['darwin_x64']);
  assert.deepEqual(candidateKeys('linux', 'x64'), ['linux_x64']);
  assert.deepEqual(candidateKeys('linux', 'arm64'), ['linux_arm64']);
});

test('prefers the package matching process.arch when it is installed', () => {
  for (const [platform, arch, pkg, bin] of [
    ['win32', 'arm64', WIN_ARM, 'skillcheck.exe'],
    ['win32', 'x64', WIN_X64, 'skillcheck.exe'],
    ['darwin', 'arm64', MAC_ARM, 'skillcheck'],
    ['darwin', 'x64', MAC_X64, 'skillcheck'],
    ['linux', 'x64', LINUX_X64, 'skillcheck'],
    ['linux', 'arm64', LINUX_ARM, 'skillcheck'],
  ]) {
    // Every package present: the native one must win.
    const got = resolveBinary({ platform, arch, resolve: installed(...ALL) });
    assert.equal(got.path, path.join('/nm', pkg, bin), `${platform}/${arch}`);
  }
});

test('win32: x64 Node on ARM64 Windows falls back to the arm64 package', () => {
  const got = resolveBinary({ platform: 'win32', arch: 'x64', resolve: installed(WIN_ARM) });
  assert.equal(got.path, path.join('/nm', WIN_ARM, 'skillcheck.exe'));
});

test('win32: arm64 Node falls back to the x64 package', () => {
  const got = resolveBinary({ platform: 'win32', arch: 'arm64', resolve: installed(WIN_X64) });
  assert.equal(got.path, path.join('/nm', WIN_X64, 'skillcheck.exe'));
});

test('darwin: arm64 Node falls back to the x64 package (Rosetta)', () => {
  const got = resolveBinary({ platform: 'darwin', arch: 'arm64', resolve: installed(MAC_X64) });
  assert.equal(got.path, path.join('/nm', MAC_X64, 'skillcheck'));
});

test('darwin: x64 Node does not fall back to arm64', () => {
  const got = resolveBinary({ platform: 'darwin', arch: 'x64', resolve: installed(MAC_ARM) });
  assert.equal(got.path, undefined);
  assert.match(got.error, new RegExp(`${MAC_X64} is not installed\\.`));
  assert.doesNotMatch(got.error, /fallback/);
});

test('linux stays strict: no cross-arch fallback', () => {
  const got = resolveBinary({ platform: 'linux', arch: 'x64', resolve: installed(LINUX_ARM) });
  assert.equal(got.path, undefined);
  assert.match(got.error, new RegExp(`${LINUX_X64} is not installed\\.`));
  assert.doesNotMatch(got.error, new RegExp(LINUX_ARM));
});

test('names both packages when neither is installed', () => {
  for (const [platform, arch, preferred, fallback] of [
    ['win32', 'arm64', WIN_ARM, WIN_X64],
    ['win32', 'x64', WIN_X64, WIN_ARM],
    ['darwin', 'arm64', MAC_ARM, MAC_X64],
  ]) {
    const got = resolveBinary({ platform, arch, resolve: installed() });
    assert.equal(got.path, undefined);
    assert.ok(got.error.includes(`${preferred} is not installed`), got.error);
    assert.ok(got.error.includes(`neither is the fallback ${fallback}`), got.error);
    assert.ok(got.error.includes(`fix: npm install ${preferred}`), got.error);
  }
});

test('unsupported platform lists the supported ones', () => {
  const got = resolveBinary({ platform: 'freebsd', arch: 'x64', resolve: installed() });
  assert.match(got.error, /no prebuilt binary for freebsd\/x64/);
  assert.match(got.error, /supported: darwin_arm64/);
  const ia32 = resolveBinary({ platform: 'win32', arch: 'ia32', resolve: installed(WIN_X64) });
  assert.match(ia32.error, /no prebuilt binary for win32\/ia32/);
});

test('requiring the launcher exports it without running it', () => {
  // If require() ran main(), this test process would have spawned or exited.
  assert.equal(typeof resolveBinary, 'function');
  assert.equal(typeof candidateKeys, 'function');
});
