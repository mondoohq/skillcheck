// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Unit tests for the Slack release message builder.
//
// Run: node --test scripts/slack-release-message.test.mjs

import assert from 'node:assert/strict';
import test from 'node:test';

import { STAGES, buildFinal, buildStart, outcome, stageText } from './slack-release-message.mjs';

const base = {
  SLACK_CHANNEL: 'C123',
  SLACK_TS: '1700000000.000100',
  RELEASE_VERSION: 'v0.3.1',
  RELEASE_MODE: 'release',
  GITHUB_REPOSITORY: 'mondoohq/skillcheck',
  GITHUB_SERVER_URL: 'https://github.com',
  GITHUB_RUN_ID: '42',
  GITHUB_ACTOR: 'octocat',
  GITHUB_EVENT_NAME: 'push',
};

const allSuccess = { RESULT_BUILD: 'success', RESULT_NPM_PUBLISH: 'success', RESULT_NPM_VERIFY: 'success' };

// fieldTexts flattens every mrkdwn field and text in a payload, for assertions.
function texts(payload) {
  return payload.blocks.flatMap((b) => [
    ...(b.text ? [b.text.text] : []),
    ...(b.fields || []).map((f) => f.text),
    ...(b.elements || []).map((e) => e.text),
  ]);
}

test('start message lists every stage as pending and links the run', () => {
  const p = buildStart(base);
  assert.equal(p.channel, 'C123');
  assert.equal(p.ts, undefined, 'a new message has no ts');
  assert.equal(p.blocks[0].text.text, 'skillcheck v0.3.1 release started');
  const t = texts(p);
  for (const s of STAGES) assert.ok(t.includes(`*${s.label}:*\n:hourglass_flowing_sand: Pending`), s.label);
  assert.ok(t.includes('<https://github.com/mondoohq/skillcheck/actions/runs/42|Open workflow run>'));
});

test('start message for a republish marks the build as not needed', () => {
  const p = buildStart({ ...base, RELEASE_MODE: 'republish', RELEASE_VERSION: '0.3.0' });
  assert.equal(p.blocks[0].text.text, 'skillcheck 0.3.0 npm republish started');
  assert.ok(texts(p).includes('*GoReleaser build:*\n:double_vertical_bar: Not needed (republish)'));
});

test('final message on success updates the same message and links release and npm', () => {
  const p = buildFinal({ ...base, ...allSuccess, SLACK_ALERT_GROUP: 'S999' });
  assert.equal(p.ts, base.SLACK_TS);
  assert.equal(p.blocks[0].text.text, 'skillcheck v0.3.1 release completed ✓');
  const summary = p.blocks[1].text.text;
  assert.match(summary, /skillcheck 0\.3\.1 is live/);
  assert.match(summary, /releases\/tag\/v0\.3\.1/);
  assert.match(summary, /npmjs\.com\/package\/@mondoohq\/skillcheck\/v\/0\.3\.1/);
  assert.doesNotMatch(p.text, /subteam/, 'no alert on success');
});

test('final message on failure mentions the alert group and marks the failed stage', () => {
  const p = buildFinal({ ...base, ...allSuccess, RESULT_NPM_PUBLISH: 'failure', RESULT_NPM_VERIFY: 'skipped', SLACK_ALERT_GROUP: 'S999' });
  assert.equal(p.blocks[0].text.text, 'skillcheck v0.3.1 release needs attention');
  assert.match(p.text, /^<!subteam\^S999> /);
  const t = texts(p);
  assert.ok(t.includes('*npm publish:*\n:x: Failed'));
  assert.ok(t.includes('*npm install check:*\n:double_vertical_bar: Not run'));
});

test('final message on failure without an alert group has no mention', () => {
  const p = buildFinal({ ...base, ...allSuccess, RESULT_BUILD: 'failure' });
  assert.doesNotMatch(p.text, /subteam/);
});

test('a successful republish counts as success despite the skipped build', () => {
  const env = { ...base, ...allSuccess, RELEASE_MODE: 'republish', RESULT_BUILD: 'skipped' };
  assert.equal(buildFinal(env).blocks[0].text.text, 'skillcheck v0.3.1 npm republish completed ✓');
});

test('a skipped build on a tag release is not success', () => {
  assert.equal(outcome({ build: 'skipped', npmPublish: 'success', npmVerify: 'success' }, 'release'), 'failure');
});

test('cancellation is reported as cancelled, without an alert', () => {
  const p = buildFinal({ ...base, ...allSuccess, RESULT_NPM_VERIFY: 'cancelled', SLACK_ALERT_GROUP: 'S999' });
  assert.equal(p.blocks[0].text.text, 'skillcheck v0.3.1 release cancelled');
  assert.doesNotMatch(p.text, /subteam/);
});

test('an unknown or missing result is shown, not hidden', () => {
  assert.match(stageText(STAGES[0], undefined, 'release'), /Unknown \(no result\)/);
});
