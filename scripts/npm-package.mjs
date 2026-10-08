// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Builds (and optionally publishes) the npm packages for a release.
//
// Seven packages ship per release: six `@mondoohq/skillcheck_<os>_<arch>`
// packages, each holding one prebuilt binary, and the `@mondoohq/skillcheck`
// wrapper (npm/) that declares them as optionalDependencies and runs whichever
// one npm installed.
//
// Usage:
//   node scripts/npm-package.mjs --version 1.2.3 [--dist dist] [--out dist/npm] [--from-release] [--publish]
//
// By default the binaries come from goreleaser's dist/artifacts.json. With
// --from-release they come from the archives attached to the version's GitHub
// release instead, which republishes an already-released version without
// rebuilding it. Without --publish the packages are only laid out.

import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath, pathToFileURL } from 'node:url';

// The source of truth for which packages exist. npm/bin/skillcheck.js and
// scripts/verify-npm-release.sh both key off this list.
export const PLATFORMS = [
  { goos: 'darwin', goarch: 'amd64', os: 'darwin', cpu: 'x64', bin: 'skillcheck' },
  { goos: 'darwin', goarch: 'arm64', os: 'darwin', cpu: 'arm64', bin: 'skillcheck' },
  { goos: 'linux', goarch: 'amd64', os: 'linux', cpu: 'x64', bin: 'skillcheck' },
  { goos: 'linux', goarch: 'arm64', os: 'linux', cpu: 'arm64', bin: 'skillcheck' },
  { goos: 'windows', goarch: 'amd64', os: 'win32', cpu: 'x64', bin: 'skillcheck.exe' },
  { goos: 'windows', goarch: 'arm64', os: 'win32', cpu: 'arm64', bin: 'skillcheck.exe' },
];

const SCOPE = '@mondoohq';
export const WRAPPER_NAME = `${SCOPE}/skillcheck`;
const BUILDER_ID = 'skillcheck'; // builds[].id in .goreleaser.yaml
const REPO_URL = 'git+https://github.com/mondoohq/skillcheck.git';

export function packageNameFor({ goos, goarch }) {
  return `${SCOPE}/skillcheck_${goos}_${goarch}`;
}

// selectBinaries maps goreleaser's artifacts.json onto PLATFORMS. A missing or
// duplicated target fails the build: a wrapper whose optionalDependency does
// not exist installs "successfully" and then cannot find its binary.
export function selectBinaries(artifacts) {
  const binaries = artifacts.filter(
    (a) => a.type === 'Binary' && a.extra?.ID === BUILDER_ID
  );
  return PLATFORMS.map((p) => {
    const hits = binaries.filter((a) => a.goos === p.goos && a.goarch === p.goarch);
    if (hits.length !== 1) {
      throw new Error(
        `expected exactly one ${BUILDER_ID} binary for ${p.goos}/${p.goarch} in artifacts.json, found ${hits.length}`
      );
    }
    return { ...p, artifact: hits[0] };
  });
}

function writeJSON(file, value) {
  fs.writeFileSync(file, JSON.stringify(value, null, 2) + '\n');
}

// copyDocs puts the repo README and LICENSE into a package.
function copyDocs(repoRoot, dir) {
  for (const f of ['README.md', 'LICENSE']) {
    fs.copyFileSync(path.join(repoRoot, f), path.join(dir, f));
  }
}

// layoutPlatformPackage lays out one platform package from a binary already on
// disk. Both the goreleaser path and the --from-release path go through here,
// so they cannot disagree on layout. `os` and `cpu` are what make npm install
// exactly one of the six.
function layoutPlatformPackage({ target, version, binSrc, outDir, repoRoot }) {
  const name = packageNameFor(target);
  const dir = path.join(outDir, ...name.split('/'));
  fs.mkdirSync(dir, { recursive: true });

  fs.copyFileSync(binSrc, path.join(dir, target.bin));
  // Actions artifacts and some extractors drop the exec bit, and npm packs the
  // mode it finds.
  fs.chmodSync(path.join(dir, target.bin), 0o755);

  writeJSON(path.join(dir, 'package.json'), {
    name,
    version,
    description: `skillcheck binary for ${target.os} ${target.cpu}. Install @mondoohq/skillcheck instead.`,
    homepage: 'https://github.com/mondoohq/skillcheck#readme',
    repository: { type: 'git', url: REPO_URL },
    license: 'Apache-2.0',
    os: [target.os],
    cpu: [target.cpu],
    files: [target.bin, 'README.md', 'LICENSE'],
    preferUnplugged: true,
  });
  copyDocs(repoRoot, dir);
  return dir;
}

// buildWrapperPackage copies npm/ and stamps the version into it and into every
// optionalDependency, pinned exactly so a wrapper never pairs with a platform
// package from another release.
function buildWrapperPackage({ version, repoRoot, outDir }) {
  const dir = path.join(outDir, ...WRAPPER_NAME.split('/'));
  fs.mkdirSync(path.join(dir, 'bin'), { recursive: true });

  const pkg = JSON.parse(fs.readFileSync(path.join(repoRoot, 'npm', 'package.json'), 'utf8'));
  pkg.version = version;
  pkg.optionalDependencies = Object.fromEntries(
    PLATFORMS.map((p) => [packageNameFor(p), version])
  );
  writeJSON(path.join(dir, 'package.json'), pkg);

  fs.copyFileSync(
    path.join(repoRoot, 'npm', 'bin', 'skillcheck.js'),
    path.join(dir, 'bin', 'skillcheck.js')
  );
  fs.chmodSync(path.join(dir, 'bin', 'skillcheck.js'), 0o755);
  copyDocs(repoRoot, dir);
  return dir;
}

// build lays out the seven packages from goreleaser's dist. The wrapper is last.
export function build({ version, repoRoot, distDir, outDir }) {
  const artifacts = JSON.parse(fs.readFileSync(path.join(distDir, 'artifacts.json'), 'utf8'));
  const targets = selectBinaries(artifacts);
  fs.rmSync(outDir, { recursive: true, force: true });
  const dirs = targets.map((target) => {
    // artifacts.json paths are relative to the repo root (dist/<build>/<bin>).
    const binSrc = path.resolve(distDir, '..', target.artifact.path);
    return layoutPlatformPackage({ target, version, binSrc, outDir, repoRoot });
  });
  dirs.push(buildWrapperPackage({ version, repoRoot, outDir }));
  return dirs;
}

// releaseArchiveName is goreleaser's archive for a target at a version, e.g.
// skillcheck_0.3.0_darwin_arm64.tar.gz (windows is a .zip). The binary sits at
// the archive root.
export function releaseArchiveName(version, { goos, goarch }) {
  const ext = goos === 'windows' ? 'zip' : 'tar.gz';
  return `skillcheck_${version}_${goos}_${goarch}.${ext}`;
}

// buildFromRelease lays out the seven packages for an already-released version
// from the archives attached to its GitHub release, so the exact released bytes
// are published. fetchArchive(assetName, destPath) and
// extractBinary(archivePath, binName, destPath) are injected for testing.
export function buildFromRelease({ version, repoRoot, workDir, outDir, fetchArchive, extractBinary }) {
  fs.rmSync(outDir, { recursive: true, force: true });
  fs.mkdirSync(workDir, { recursive: true });

  const dirs = PLATFORMS.map((target) => {
    const asset = releaseArchiveName(version, target);
    const archive = path.join(workDir, asset);
    fetchArchive(asset, archive);
    const binSrc = path.join(workDir, `${target.goos}_${target.goarch}`, target.bin);
    fs.mkdirSync(path.dirname(binSrc), { recursive: true });
    extractBinary(archive, target.bin, binSrc);
    if (!fs.existsSync(binSrc)) {
      throw new Error(`release archive ${asset} did not contain ${target.bin}`);
    }
    return layoutPlatformPackage({ target, version, binSrc, outDir, repoRoot });
  });
  dirs.push(buildWrapperPackage({ version, repoRoot, outDir }));
  return dirs;
}

function ghFetchArchive(version, assetName, destPath) {
  execFileSync(
    'gh',
    ['release', 'download', `v${version}`, '--pattern', assetName, '--output', destPath, '--clobber'],
    { stdio: 'inherit' }
  );
}

function extractReleaseBinary(archivePath, binName, destPath) {
  const dir = path.dirname(destPath);
  if (archivePath.endsWith('.zip')) {
    execFileSync('unzip', ['-o', '-j', archivePath, binName, '-d', dir], { stdio: 'inherit' });
  } else {
    execFileSync('tar', ['-xzf', archivePath, '-C', dir, binName], { stdio: 'inherit' });
  }
}

// npmIsPublished asks the registry whether name@version exists. `npm view`
// exits non-zero (E404) for an absent version, read here as "not there".
function npmIsPublished(name, version) {
  try {
    const out = execFileSync('npm', ['view', `${name}@${version}`, 'version'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();
    return out === version;
  } catch {
    return false;
  }
}

// npmPublishDir publishes one package. Auth is npm trusted publishing (OIDC
// from the workflow's id-token), which also attaches provenance. --access
// public because @mondoohq packages default to restricted.
function npmPublishDir(dir) {
  execFileSync('npm', ['publish', '--access', 'public', '--provenance'], {
    cwd: dir,
    stdio: 'inherit',
  });
}

function sleepSeconds(secs) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, Math.max(0, secs) * 1000);
}

// publish ships the packages in the one order that never leaves an install
// broken:
//
//   1. publish the six platform packages,
//   2. wait until all six are live on the registry,
//   3. only then publish the wrapper.
//
// The wrapper pins each platform package to an exact version, so once it is
// resolvable an install fails with `notarget` for any platform package that
// has not propagated yet. `npm publish` exiting 0 means npm accepted the
// tarball, not that it is visible; propagation is asynchronous. If a platform
// package never shows up by the deadline this throws before the wrapper is
// published.
//
// Idempotent: a package already on the registry is skipped, so a re-run (or a
// --from-release recovery) publishes only what is missing.
export function publish(dirs, opts = {}) {
  const {
    isPublished = npmIsPublished,
    publishDir = npmPublishDir,
    sleep = sleepSeconds,
    now = () => Date.now(),
    deadlineSecs = Number(process.env.NPM_PUBLISH_DEADLINE ?? 3600),
    intervalSecs = Number(process.env.NPM_PUBLISH_INTERVAL ?? 30),
    log = (line) => process.stdout.write(line + '\n'),
  } = opts;

  const pkgs = dirs.map((dir) => {
    const { name, version } = JSON.parse(fs.readFileSync(path.join(dir, 'package.json'), 'utf8'));
    return { dir, name, version };
  });
  const platforms = pkgs.filter((p) => p.name !== WRAPPER_NAME);
  const wrappers = pkgs.filter((p) => p.name === WRAPPER_NAME);

  const publishOne = (p) => {
    if (isPublished(p.name, p.version)) {
      log(`skip ${p.name}@${p.version} (already on registry)`);
      return;
    }
    log(`publishing ${p.name}@${p.version}`);
    publishDir(p.dir);
  };

  for (const p of platforms) publishOne(p);

  const deadline = now() + deadlineSecs * 1000;
  const live = new Set();
  for (;;) {
    for (const p of platforms) {
      if (!live.has(p.name) && isPublished(p.name, p.version)) live.add(p.name);
    }
    if (live.size === platforms.length) break;
    if (now() >= deadline) {
      const missing = platforms
        .filter((p) => !live.has(p.name))
        .map((p) => `${p.name}@${p.version}`)
        .join(', ');
      throw new Error(
        `after ${deadlineSecs}s these platform packages are still not live: ${missing}. ` +
          `Not publishing ${WRAPPER_NAME}, whose pinned optionalDependencies would not resolve. ` +
          'Re-running publishes only what is missing.'
      );
    }
    log(`waiting for ${platforms.length - live.size} platform package(s) to go live; re-checking in ${intervalSecs}s`);
    sleep(intervalSecs);
  }
  log(`all ${platforms.length} platform packages are live`);

  for (const p of wrappers) publishOne(p);
}

export function parseArgs(argv) {
  const args = { dist: 'dist', out: path.join('dist', 'npm'), publish: false, fromRelease: false };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--publish') args.publish = true;
    else if (a === '--from-release') args.fromRelease = true;
    else if (a === '--version') args.version = argv[++i];
    else if (a === '--dist') args.dist = argv[++i];
    else if (a === '--out') args.out = argv[++i];
    else throw new Error(`unknown argument ${a}`);
  }
  if (!args.version) throw new Error('--version is required');
  // The tag spells it with a leading v; package.json does not.
  args.version = args.version.replace(/^v/, '');
  if (!/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/.test(args.version)) {
    throw new Error(`--version ${args.version} is not a semver version`);
  }
  return args;
}

// Run only when invoked directly, so tests can import the helpers.
if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  const args = parseArgs(process.argv.slice(2));
  const repoRoot = path.resolve(fileURLToPath(import.meta.url), '..', '..');
  const dirs = args.fromRelease
    ? buildFromRelease({
        version: args.version,
        repoRoot,
        workDir: path.resolve(args.dist, 'from-release'),
        outDir: path.resolve(args.out),
        fetchArchive: (asset, dest) => ghFetchArchive(args.version, asset, dest),
        extractBinary: extractReleaseBinary,
      })
    : build({
        version: args.version,
        repoRoot,
        distDir: path.resolve(args.dist),
        outDir: path.resolve(args.out),
      });
  for (const d of dirs) process.stdout.write(`built ${path.relative(repoRoot, d)}\n`);
  if (args.publish) publish(dirs);
  else process.stdout.write('not publishing (pass --publish)\n');
}
