// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Builds the Slack payloads for release notifications.
//
// A release posts one message when it starts (every stage pending) and then
// updates that same message in place with the final status of each stage. The
// payloads are built here rather than in workflow expressions so the status
// logic is readable and unit-tested.
//
// Usage (from release.yml):
//   node scripts/slack-release-message.mjs start > payload.json
//   node scripts/slack-release-message.mjs final > payload.json
//
// Inputs come from the environment:
//   SLACK_CHANNEL       channel ID to post to (start), or of the message (final)
//   SLACK_TS            ts of the message to update (final only)
//   SLACK_ALERT_GROUP   optional Slack user-group ID to mention when a release needs attention
//   RELEASE_VERSION     version or tag being released, e.g. v0.3.1
//   RELEASE_MODE        "release" (tag push) or "republish" (manual npm republish)
//   RESULT_BUILD, RESULT_NPM_PUBLISH, RESULT_NPM_VERIFY
//                       job results: success | failure | cancelled | skipped
//   GITHUB_REPOSITORY, GITHUB_SERVER_URL, GITHUB_RUN_ID, GITHUB_ACTOR, GITHUB_EVENT_NAME

import path from 'node:path';
import { pathToFileURL } from 'node:url';

// STAGES are the release jobs reported in the message, in pipeline order.
export const STAGES = [
  { key: 'build', label: 'GoReleaser build', done: 'Built & GitHub release created' },
  { key: 'npmPublish', label: 'npm publish', done: 'Published to npm' },
  { key: 'npmVerify', label: 'npm install check', done: 'Installs and exit codes verified' },
];

const ICON = {
  pending: ':hourglass_flowing_sand:',
  success: ':white_check_mark:',
  failure: ':x:',
  cancelled: ':no_entry_sign:',
  skipped: ':double_vertical_bar:',
};

// stageText renders one stage's status line.
export function stageText(stage, result, mode) {
  switch (result) {
    case 'pending':
      return `${ICON.pending} Pending`;
    case 'success':
      return `${ICON.success} ${stage.done}`;
    case 'failure':
      return `${ICON.failure} Failed`;
    case 'cancelled':
      return `${ICON.cancelled} Cancelled`;
    case 'skipped':
      // A republish reuses the binaries of an existing GitHub release.
      if (stage.key === 'build' && mode === 'republish') return `${ICON.skipped} Not needed (republish)`;
      return `${ICON.skipped} Not run`;
    default:
      return `${ICON.failure} Unknown (${result || 'no result'})`;
  }
}

// outcome classifies the release from its stage results. A stage is fine if it
// succeeded, or if it is the build being skipped on a republish.
export function outcome(results, mode) {
  const ok = STAGES.every(
    (s) => results[s.key] === 'success' || (s.key === 'build' && mode === 'republish' && results[s.key] === 'skipped')
  );
  if (ok) return 'success';
  if (STAGES.some((s) => results[s.key] === 'cancelled')) return 'cancelled';
  return 'failure';
}

function context(env) {
  const repo = env.GITHUB_REPOSITORY || 'mondoohq/skillcheck';
  const server = env.GITHUB_SERVER_URL || 'https://github.com';
  const tag = env.RELEASE_VERSION || 'unknown';
  const version = tag.replace(/^v/, '');
  return {
    repo,
    name: repo.split('/').pop(),
    tag,
    version,
    mode: env.RELEASE_MODE === 'republish' ? 'republish' : 'release',
    actor: env.GITHUB_ACTOR || 'unknown',
    trigger: env.GITHUB_EVENT_NAME || 'unknown',
    runUrl: `${server}/${repo}/actions/runs/${env.GITHUB_RUN_ID || ''}`,
    releaseUrl: `${server}/${repo}/releases/tag/v${version}`,
    npmUrl: `https://www.npmjs.com/package/@mondoohq/${repo.split('/').pop()}/v/${version}`,
    channel: env.SLACK_CHANNEL || '',
    ts: env.SLACK_TS || '',
    alertGroup: env.SLACK_ALERT_GROUP || '',
  };
}

function detailFields(c) {
  return {
    type: 'section',
    fields: [
      { type: 'mrkdwn', text: `*Repository:*\n\`${c.repo}\`` },
      { type: 'mrkdwn', text: `*Version:*\n\`${c.tag}\`` },
      { type: 'mrkdwn', text: `*Mode:*\n${c.mode === 'republish' ? 'npm republish' : 'Release'}` },
      { type: 'mrkdwn', text: `*Triggered by:*\n${c.actor} (${c.trigger})` },
    ],
  };
}

function stageFields(results, mode) {
  return {
    type: 'section',
    fields: STAGES.map((s) => ({ type: 'mrkdwn', text: `*${s.label}:*\n${stageText(s, results[s.key], mode)}` })),
  };
}

function runLink(c) {
  return { type: 'context', elements: [{ type: 'mrkdwn', text: `<${c.runUrl}|Open workflow run>` }] };
}

// buildStart is the message posted when the release begins.
export function buildStart(env) {
  const c = context(env);
  const pending = Object.fromEntries(STAGES.map((s) => [s.key, 'pending']));
  if (c.mode === 'republish') pending.build = 'skipped';
  const what = c.mode === 'republish' ? 'npm republish' : 'release';
  return {
    channel: c.channel,
    unfurl_links: false,
    unfurl_media: false,
    text: `${c.name} ${c.tag} ${what} started`,
    blocks: [
      { type: 'header', text: { type: 'plain_text', text: `${c.name} ${c.tag} ${what} started`, emoji: true } },
      detailFields(c),
      stageFields(pending, c.mode),
      runLink(c),
    ],
  };
}

// buildFinal is the update applied to the start message when the release ends.
export function buildFinal(env) {
  const c = context(env);
  const results = {
    build: env.RESULT_BUILD,
    npmPublish: env.RESULT_NPM_PUBLISH,
    npmVerify: env.RESULT_NPM_VERIFY,
  };
  const result = outcome(results, c.mode);
  const what = c.mode === 'republish' ? 'npm republish' : 'release';
  const mention = result === 'failure' && c.alertGroup ? `<!subteam^${c.alertGroup}> ` : '';

  let header;
  let summary;
  if (result === 'success') {
    header = `${c.name} ${c.tag} ${what} completed ✓`;
    summary = `:white_check_mark: *${c.name} ${c.version} is live.* <${c.releaseUrl}|GitHub release> · <${c.npmUrl}|npm>`;
  } else if (result === 'cancelled') {
    header = `${c.name} ${c.tag} ${what} cancelled`;
    summary = ':no_entry_sign: *The release was cancelled before it finished.* Re-run the workflow or republish the version.';
  } else {
    header = `${c.name} ${c.tag} ${what} needs attention`;
    summary = `:rotating_light: ${mention}*The ${what} did not complete.* Check the failed stage below.`;
  }

  return {
    channel: c.channel,
    ts: c.ts,
    text: `${mention}${header}`,
    blocks: [
      { type: 'header', text: { type: 'plain_text', text: header, emoji: true } },
      { type: 'section', text: { type: 'mrkdwn', text: summary } },
      detailFields(c),
      stageFields(results, c.mode),
      runLink(c),
    ],
  };
}

// Run only when invoked directly, so tests can import the builders.
if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  const kind = process.argv[2];
  const build = { start: buildStart, final: buildFinal }[kind];
  if (!build) {
    process.stderr.write('usage: slack-release-message.mjs start|final\n');
    process.exit(2);
  }
  process.stdout.write(JSON.stringify(build(process.env), null, 2) + '\n');
}
