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

// resolveBinary finds the platform binary, or explains why it cannot. Each
// failure is something the user can act on, so each gets a sentence rather than
// a module-resolution stack trace.
function resolveBinary() {
  const key = `${process.platform}_${process.arch}`;
  const target = PLATFORM_PACKAGES[key];
  if (!target) {
    const supported = Object.keys(PLATFORM_PACKAGES).sort().join(', ');
    fail(
      `no prebuilt binary for ${process.platform}/${process.arch}.\n` +
        `  supported: ${supported}\n` +
        '  other platforms: https://github.com/mondoohq/skillcheck/releases'
    );
  }
  let pkgJson;
  try {
    pkgJson = require.resolve(path.posix.join(target.pkg, 'package.json'));
  } catch {
    fail(
      `the platform package ${target.pkg} is not installed.\n` +
        '  it ships as an optionalDependency, so this usually means the install ran with\n' +
        '  --no-optional / --omit=optional, or a lockfile from a different platform was used.\n' +
        `  fix: npm install ${target.pkg}`
    );
  }
  return path.join(path.dirname(pkgJson), target.bin);
}

function main() {
  const child = spawn(resolveBinary(), process.argv.slice(2), {
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

main();
