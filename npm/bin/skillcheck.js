#!/usr/bin/env node
// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Launcher for the `skillcheck` command installed from npm.
//
// npm cannot ship one package with six platform binaries, so
// `@mondoohq/skillcheck` is a thin wrapper: it declares the six
// `@mondoohq/skillcheck_<os>_<arch>` packages as optionalDependencies (npm
// installs only the one matching `os`/`cpu`) and this script runs whichever one
// landed.
//
// Its job is to be invisible, which takes two things:
//
//   - THE EXIT CODE. skillcheck exits 1 when a scan finds critical/high-risk
//     skills or `skillcheck validate` fails a check, and CI gates on that. A
//     launcher that spawns the binary without reading its status exits 0
//     always, which silently disables every such gate for npm users.
//   - SIGNALS. Without forwarding, Ctrl+C kills the launcher and leaves the
//     binary running detached.

'use strict';

const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

// The platform packages, keyed by `${process.platform}_${process.arch}`.
// Keep in sync with PLATFORMS in scripts/npm-package.mjs.
const PLATFORM_PACKAGES = {
  linux_x64: { pkg: '@mondoohq/skillcheck_linux_amd64', bin: 'skillcheck' },
  linux_arm64: { pkg: '@mondoohq/skillcheck_linux_arm64', bin: 'skillcheck' },
  darwin_x64: { pkg: '@mondoohq/skillcheck_darwin_amd64', bin: 'skillcheck' },
  darwin_arm64: { pkg: '@mondoohq/skillcheck_darwin_arm64', bin: 'skillcheck' },
  win32_x64: { pkg: '@mondoohq/skillcheck_windows_amd64', bin: 'skillcheck.exe' },
  win32_arm64: { pkg: '@mondoohq/skillcheck_windows_arm64', bin: 'skillcheck.exe' },
};

// A launcher failure must never look like success.
const EXIT_FAILURE = 1;

function fail(message) {
  process.stderr.write(`skillcheck: ${message}\n`);
  process.exit(EXIT_FAILURE);
}

// candidateKeys lists the PLATFORM_PACKAGES keys to try, most preferred first.
//
// npm installs the optionalDependency matching the CPU of the Node that ran
// `npm install`, not the machine's, so switching Node builds afterwards leaves
// the "wrong" package on disk. Where the OS can run the other CPU's binary
// anyway, that package is an acceptable fallback:
//
//   - win32: Windows on ARM64 runs both arm64 and x64 binaries (x64 under
//     emulation), and an x64 Node there is common. arm64 <-> x64 both ways.
//   - darwin: Apple Silicon runs x64 binaries under Rosetta, so an arm64 Node
//     may use the x64 package. Not the reverse: an x64 Node may be on an Intel
//     Mac, which cannot run arm64.
//   - linux: strict. There is no general cross-CPU execution, and a binary
//     that fails to exec is worse than a clear "not installed".
function candidateKeys(platform, arch) {
  const keys = [`${platform}_${arch}`];
  if (platform === 'win32') {
    if (arch === 'arm64') keys.push('win32_x64');
    else if (arch === 'x64') keys.push('win32_arm64');
  } else if (platform === 'darwin' && arch === 'arm64') {
    keys.push('darwin_x64');
  }
  return keys;
}

// resolveBinary finds the platform binary, or explains why it cannot. Each
// failure is something the user can act on, so each gets a sentence rather than
// a module-resolution stack trace.
//
// platform, arch and resolve default to the real process and require.resolve;
// tests inject them. Returns { path } on success or { error } with the message.
function resolveBinary({ platform = process.platform, arch = process.arch, resolve = require.resolve } = {}) {
  const targets = candidateKeys(platform, arch)
    .map((key) => PLATFORM_PACKAGES[key])
    .filter(Boolean);
  if (targets.length === 0) {
    const supported = Object.keys(PLATFORM_PACKAGES).sort().join(', ');
    return {
      error:
        `no prebuilt binary for ${platform}/${arch}.\n` +
        `  supported: ${supported}\n` +
        '  other platforms: https://github.com/mondoohq/skillcheck/releases',
    };
  }
  for (const target of targets) {
    let pkgJson;
    try {
      pkgJson = resolve(path.posix.join(target.pkg, 'package.json'));
    } catch {
      continue;
    }
    return { path: path.join(path.dirname(pkgJson), target.bin) };
  }
  const preferred = targets[0].pkg;
  const fallbacks = targets.slice(1).map((t) => t.pkg);
  const which =
    `the platform package ${preferred} is not installed` +
    (fallbacks.length ? `, and neither is the fallback ${fallbacks.join(', ')}.` : '.');
  return {
    error:
      `${which}\n` +
      '  it ships as an optionalDependency, so this usually means the install ran with\n' +
      '  --no-optional / --omit=optional, or a lockfile from a different platform was used,\n' +
      `  or node_modules was installed by a Node build for a different CPU than this one (${arch}).\n` +
      `  fix: npm install ${preferred}`,
  };
}

function main() {
  const resolved = resolveBinary();
  if (resolved.error) fail(resolved.error);
  const child = spawn(resolved.path, process.argv.slice(2), {
    stdio: 'inherit',
    env: process.env,
  });

  // Forward signals so Ctrl+C reaches the binary instead of orphaning it.
  const forwarded = ['SIGINT', 'SIGTERM', 'SIGHUP', 'SIGQUIT', 'SIGBREAK'];
  const handlers = [];
  for (const signal of forwarded) {
    // Not every platform has every signal (Windows has no SIGHUP/SIGQUIT).
    if (!(signal in os.constants.signals)) continue;
    const handler = () => {
      try {
        child.kill(signal);
      } catch {
        /* already exited */
      }
    };
    handlers.push([signal, handler]);
    process.on(signal, handler);
  }

  child.on('error', (err) => fail(`could not run the skillcheck binary: ${err.message}`));

  child.on('close', (code, signal) => {
    for (const [sig, handler] of handlers) process.removeListener(sig, handler);

    if (signal) {
      // Killed rather than exited: report it the way a shell does (128+signum).
      const signum = os.constants.signals[signal];
      process.exit(signum ? 128 + signum : EXIT_FAILURE);
    }
    process.exit(code === null ? EXIT_FAILURE : code);
  });
}

// Run when executed as the `skillcheck` bin; export the resolution logic when
// required, so scripts/npm-launcher.test.mjs can test it without spawning.
if (require.main === module) {
  main();
} else {
  module.exports = { PLATFORM_PACKAGES, candidateKeys, resolveBinary };
}
