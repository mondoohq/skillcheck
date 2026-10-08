// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Unit tests for the npm packaging and publish path, plus the launcher's exit
// contract. The registry check, publish call and sleep are injected, so these
// run with no npm and no network.
//
// Run: node --test scripts/npm-package.test.mjs

import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

import {
  PLATFORMS,
  WRAPPER_NAME,
  build,
  buildFromRelease,
  packageNameFor,
  parseArgs,
  publish,
  releaseArchiveName,
  selectBinaries,
} from './npm-package.mjs';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const tmp = (prefix) => fs.mkdtempSync(path.join(os.tmpdir(), prefix));
const quiet = { log: () => {} };

function readPkg(dir) {
  return JSON.parse(fs.readFileSync(path.join(dir, 'package.json'), 'utf8'));
}

// fakeDist writes a goreleaser-shaped dist/ with one stub binary per platform.
function fakeDist() {
  const root = tmp('skc-dist-');
  const distDir = path.join(root, 'dist');
  const artifacts = PLATFORMS.map((p) => {
    const rel = path.join('dist', `skillcheck_${p.goos}_${p.goarch}`, p.bin);
    fs.mkdirSync(path.dirname(path.join(root, rel)), { recursive: true });
    fs.writeFileSync(path.join(root, rel), 'stub');
    return { type: 'Binary', path: rel, goos: p.goos, goarch: p.goarch, extra: { ID: 'skillcheck' } };
  });
  fs.writeFileSync(path.join(distDir, 'artifacts.json'), JSON.stringify(artifacts));
  return { distDir, artifacts };
}

// layout writes a throwaway package dir holding just the fields publish() reads.
function layout(name, version) {
  const dir = tmp('skc-pkg-');
  fs.writeFileSync(path.join(dir, 'package.json'), JSON.stringify({ name, version }));
  return dir;
}

const noWait = { sleep: () => {}, now: () => 0, deadlineSecs: 600, intervalSecs: 1, ...quiet };

test('the launcher knows every platform package the packager builds', () => {
  const launcher = fs.readFileSync(path.join(repoRoot, 'npm', 'bin', 'skillcheck.js'), 'utf8');
  for (const p of PLATFORMS) assert.match(launcher, new RegExp(`'${packageNameFor(p)}'`));
});

test('selectBinaries fails on a missing target', () => {
  const { artifacts } = fakeDist();
  assert.throws(() => selectBinaries(artifacts.slice(1)), /darwin\/amd64.*found 0/);
});

test('selectBinaries fails on a duplicated target', () => {
  const { artifacts } = fakeDist();
  assert.throws(() => selectBinaries([...artifacts, artifacts[0]]), /found 2/);
});

test('build lays out six platform packages and a wrapper pinned to the exact version', () => {
  const { distDir } = fakeDist();
  const outDir = tmp('skc-out-');
  const dirs = build({ version: '1.2.3', repoRoot, distDir, outDir });
  assert.equal(dirs.length, 7);

  for (const p of PLATFORMS) {
    const dir = path.join(outDir, ...packageNameFor(p).split('/'));
    const pkg = readPkg(dir);
    assert.equal(pkg.version, '1.2.3');
    assert.deepEqual(pkg.os, [p.os]);
    assert.deepEqual(pkg.cpu, [p.cpu]);
    assert.ok(pkg.files.includes(p.bin));
    if (process.platform !== 'win32') {
      assert.equal(fs.statSync(path.join(dir, p.bin)).mode & 0o111, 0o111, `${p.bin} is executable`);
    }
    for (const f of ['README.md', 'LICENSE']) assert.ok(fs.existsSync(path.join(dir, f)), `${f} shipped`);
  }

  const wrapperDir = dirs[dirs.length - 1];
  const wrapper = readPkg(wrapperDir);
  assert.equal(wrapper.name, WRAPPER_NAME);
  assert.equal(wrapper.version, '1.2.3');
  assert.deepEqual(
    wrapper.optionalDependencies,
    Object.fromEntries(PLATFORMS.map((p) => [packageNameFor(p), '1.2.3']))
  );
  assert.ok(fs.existsSync(path.join(wrapperDir, 'bin', 'skillcheck.js')));
});

test('releaseArchiveName: windows is a .zip, the rest .tar.gz', () => {
  assert.equal(releaseArchiveName('0.3.0', { goos: 'darwin', goarch: 'arm64' }), 'skillcheck_0.3.0_darwin_arm64.tar.gz');
  assert.equal(releaseArchiveName('0.3.0', { goos: 'windows', goarch: 'amd64' }), 'skillcheck_0.3.0_windows_amd64.zip');
});

test('buildFromRelease repackages the release archives', () => {
  const fetched = [];
  const dirs = buildFromRelease({
    version: '0.3.0',
    repoRoot,
    workDir: tmp('skc-work-'),
    outDir: tmp('skc-out-'),
    fetchArchive: (asset, dest) => {
      fetched.push(asset);
      fs.writeFileSync(dest, 'archive');
    },
    extractBinary: (_archive, _bin, dest) => fs.writeFileSync(dest, 'stub'),
  });
  assert.deepEqual(fetched.sort(), PLATFORMS.map((p) => releaseArchiveName('0.3.0', p)).sort());
  assert.equal(dirs.length, 7);
  assert.equal(readPkg(dirs[6]).name, WRAPPER_NAME);
});

test('buildFromRelease fails when an archive lacks the binary', () => {
  assert.throws(
    () =>
      buildFromRelease({
        version: '0.3.0',
        repoRoot,
        workDir: tmp('skc-work-'),
        outDir: tmp('skc-out-'),
        fetchArchive: (_asset, dest) => fs.writeFileSync(dest, 'archive'),
        extractBinary: () => {},
      }),
    /did not contain/
  );
});

test('parseArgs strips the tag v and rejects non-semver versions', () => {
  assert.equal(parseArgs(['--version', 'v0.3.0']).version, '0.3.0');
  assert.equal(parseArgs(['--version', '1.0.0-rc.1']).version, '1.0.0-rc.1');
  assert.throws(() => parseArgs(['--version', '0.3']), /not a semver/);
  assert.throws(() => parseArgs(['--version', '0.3.0; rm -rf /']), /not a semver/);
  assert.throws(() => parseArgs([]), /required/);
});

test('publish skips packages already on the registry', () => {
  const dirs = [layout('@mondoohq/skillcheck_linux_amd64', '1.0.0'), layout(WRAPPER_NAME, '1.0.0')];
  const published = [];
  publish(dirs, { ...noWait, isPublished: () => true, publishDir: (d) => published.push(d) });
  assert.deepEqual(published, []);
});

test('publish republishes only what is missing', () => {
  const present = layout('@mondoohq/skillcheck_linux_amd64', '2.0.0');
  const missing = layout('@mondoohq/skillcheck_darwin_arm64', '2.0.0');
  const wrapper = layout(WRAPPER_NAME, '2.0.0');
  const live = new Set(['@mondoohq/skillcheck_linux_amd64', WRAPPER_NAME]);
  const published = [];
  publish([present, missing, wrapper], {
    ...noWait,
    isPublished: (name) => live.has(name),
    publishDir: (d) => {
      published.push(d);
      live.add(readPkg(d).name);
    },
  });
  assert.deepEqual(published, [missing]);
});

test('publish holds the wrapper until every platform package is live', () => {
  const lagging = layout('@mondoohq/skillcheck_darwin_amd64', '3.0.0');
  const fast = layout('@mondoohq/skillcheck_linux_amd64', '3.0.0');
  const wrapper = layout(WRAPPER_NAME, '3.0.0');
  const live = new Set();
  const order = [];
  let laggingPolls = 0;
  publish([lagging, fast, wrapper], {
    ...noWait,
    isPublished: (name) => {
      if (name === '@mondoohq/skillcheck_darwin_amd64' && order.includes(name) && ++laggingPolls >= 3) {
        live.add(name);
      }
      return live.has(name);
    },
    publishDir: (d) => {
      const { name } = readPkg(d);
      order.push(name);
      if (name !== '@mondoohq/skillcheck_darwin_amd64') live.add(name);
    },
  });
  assert.ok(laggingPolls >= 3, 'the gate waited for the lagging package');
  assert.equal(order[order.length - 1], WRAPPER_NAME, 'wrapper published last');
});

test('publish never publishes the wrapper if a platform package does not go live', () => {
  const doomed = layout('@mondoohq/skillcheck_darwin_arm64', '4.0.0');
  const wrapper = layout(WRAPPER_NAME, '4.0.0');
  const order = [];
  let t = 0;
  assert.throws(
    () =>
      publish([doomed, wrapper], {
        ...quiet,
        isPublished: () => false,
        publishDir: (d) => order.push(readPkg(d).name),
        sleep: (s) => {
          t += s * 1000;
        },
        now: () => t,
        deadlineSecs: 30,
        intervalSecs: 15,
      }),
    /still not live: @mondoohq\/skillcheck_darwin_arm64@4\.0\.0/
  );
  assert.ok(!order.includes(WRAPPER_NAME));
});

test('publish surfaces a failed npm publish', () => {
  const dir = layout('@mondoohq/skillcheck_darwin_arm64', '5.0.0');
  assert.throws(
    () =>
      publish([dir], {
        ...noWait,
        isPublished: () => false,
        publishDir: () => {
          throw new Error('E404 Not Found');
        },
      }),
    /E404/
  );
});

// installLauncher lays out node_modules the way npm would for this host: the
// wrapper's launcher plus this platform's package, whose "binary" is a stub
// script that exits with the code given as its first argument.
function installLauncher() {
  const key = `${process.platform}_${process.arch}`;
  const map = { darwin: 'darwin', linux: 'linux' };
  const arch = { x64: 'amd64', arm64: 'arm64' };
  if (!map[process.platform] || !arch[process.arch]) return null;

  const root = tmp('skc-launch-');
  const wrapperBin = path.join(root, 'node_modules', '@mondoohq', 'skillcheck', 'bin');
  fs.mkdirSync(wrapperBin, { recursive: true });
  fs.copyFileSync(path.join(repoRoot, 'npm', 'bin', 'skillcheck.js'), path.join(wrapperBin, 'skillcheck.js'));

  const platDir = path.join(root, 'node_modules', '@mondoohq', `skillcheck_${map[process.platform]}_${arch[process.arch]}`);
  fs.mkdirSync(platDir, { recursive: true });
  fs.writeFileSync(path.join(platDir, 'package.json'), JSON.stringify({ name: key }));
  fs.writeFileSync(path.join(platDir, 'skillcheck'), '#!/bin/sh\nexit "${1:-0}"\n', { mode: 0o755 });
  return path.join(wrapperBin, 'skillcheck.js');
}

test('the launcher exits with the binary\'s status', { skip: process.platform === 'win32' }, (t) => {
  const launcher = installLauncher();
  if (!launcher) return t.skip(`no platform package for ${process.platform}/${process.arch}`);
  for (const code of [0, 1, 3]) {
    const res = spawnSync(process.execPath, [launcher, String(code)]);
    assert.equal(res.status, code, `binary exit ${code} survives the launcher`);
  }
});

test('the launcher fails, not succeeds, when the platform package is missing', { skip: process.platform === 'win32' }, (t) => {
  const launcher = installLauncher();
  if (!launcher) return t.skip(`no platform package for ${process.platform}/${process.arch}`);
  const modules = path.resolve(path.dirname(launcher), '..', '..');
  for (const d of fs.readdirSync(modules)) {
    if (d.startsWith('skillcheck_')) fs.rmSync(path.join(modules, d), { recursive: true });
  }
  const res = spawnSync(process.execPath, [launcher], { encoding: 'utf8' });
  assert.equal(res.status, 1);
  assert.match(res.stderr, /is not installed/);
});
