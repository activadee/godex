'use strict';

const fs = require('fs');

const OUTPUT_PATH = 'codex-output.json';

if (!fs.existsSync(OUTPUT_PATH)) {
  throw new Error('codex-output.json is missing');
}

const rawOutput = JSON.parse(fs.readFileSync(OUTPUT_PATH, 'utf8'));
const files = fs.existsSync('files.json') ? JSON.parse(fs.readFileSync('files.json', 'utf8')) : [];
const patchMap = new Map();

for (const file of files) {
  if (file && typeof file.path === 'string' && typeof file.patch === 'string') {
    patchMap.set(file.path, file.patch);
  }
}

const comments = Array.isArray(rawOutput.comments) ? rawOutput.comments : [];
const normalized = [];
const skipped = [];

const existingReviewComments = fs.existsSync('existing_review_comments.json')
  ? JSON.parse(fs.readFileSync('existing_review_comments.json', 'utf8'))
  : [];
const existingReviewThreads = fs.existsSync('existing_review_threads.json')
  ? JSON.parse(fs.readFileSync('existing_review_threads.json', 'utf8'))
  : [];

const normalizeBody = (body) => (typeof body === 'string' ? body.trim() : '');
const commentKey = (path, line, body) =>
  `${path ?? ''}#${Number.isInteger(line) ? line : 'null'}#${normalizeBody(body)}`;

const existingCommentIndex = new Map();

for (const comment of existingReviewComments) {
  if (!comment || typeof comment.path !== 'string') continue;
  const key = commentKey(comment.path, Number.isInteger(comment.line) ? comment.line : null, comment.body);
  if (!key) continue;
  existingCommentIndex.set(key, {
    resolved: false,
    author: comment.user ?? null,
    source: 'rest',
  });
}

for (const thread of existingReviewThreads) {
  if (!thread) continue;
  const resolved = Boolean(thread.resolved);
  const threadComments = Array.isArray(thread.comments) ? thread.comments : [];
  for (const comment of threadComments) {
    if (!comment || typeof comment.path !== 'string') continue;
    const line =
      Number.isInteger(comment.line) && comment.line !== null
        ? comment.line
        : null;
    const key = commentKey(comment.path, line, comment.body);
    if (!key) continue;
    const existingEntry = existingCommentIndex.get(key) ?? {
      resolved,
      author: comment.author ?? null,
      source: 'thread',
    };
    existingCommentIndex.set(key, {
      resolved: existingEntry.resolved || resolved,
      author: comment.author ?? existingEntry.author ?? null,
      source: 'thread',
      threadId: thread.id,
    });
  }
}

const newCommentKeys = new Set();

const lineIsReviewable = (patch, targetLine) => {
  if (typeof patch !== 'string' || !patch.length) return false;
  const lines = patch.split('\n');
  let newLine = 0;

  for (const diffLine of lines) {
    if (diffLine.startsWith('@@')) {
      const match = /@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@/.exec(diffLine);
      if (!match) {
        newLine = 0;
        continue;
      }
      newLine = parseInt(match[1], 10);
      continue;
    }

    if (!diffLine.length) {
      continue;
    }

    const prefix = diffLine[0];
    if (prefix === '+' || prefix === ' ') {
      if (newLine === targetLine) {
        return true;
      }
      newLine += 1;
    } else if (prefix === '-') {
      continue;
    }
  }

  return false;
};

for (const comment of comments) {
  const path = typeof comment.path === 'string' ? comment.path.trim() : '';
  const rawLine = typeof comment.line === 'number' || typeof comment.line === 'string' ? Number.parseInt(comment.line, 10) : NaN;
  const body = normalizeBody(comment.body);

  if (!path || !Number.isInteger(rawLine) || rawLine < 1 || !body) {
    skipped.push({ reason: 'invalid-shape', comment });
    continue;
  }

  if (normalized.length >= 10) {
    skipped.push({ reason: 'over-limit', path, line: rawLine, body });
    continue;
  }

  const patch = patchMap.get(path);
  if (!patch) {
    skipped.push({ reason: 'no-patch', path, line: rawLine, body });
    continue;
  }

  if (!lineIsReviewable(patch, rawLine)) {
    skipped.push({ reason: 'line-outside-diff', path, line: rawLine, body });
    continue;
  }

  const key = commentKey(path, rawLine, body);
  if (newCommentKeys.has(key)) {
    skipped.push({ reason: 'duplicate-new', path, line: rawLine, body });
    continue;
  }

  const existingEntry = existingCommentIndex.get(key);
  if (existingEntry) {
    if (existingEntry.resolved) {
      skipped.push({ reason: 'already-resolved', path, line: rawLine, body });
      continue;
    }
    skipped.push({ reason: 'already-existing', path, line: rawLine, body });
    continue;
  }

  if (normalized.some((existing) => existing.path === path && existing.line === rawLine && existing.body === body)) {
    skipped.push({ reason: 'duplicate', path, line: rawLine, body });
    continue;
  }

  normalized.push({ path, line: rawLine, side: 'RIGHT', body });
  newCommentKeys.add(key);
}

const summary =
  typeof rawOutput.summary === 'string' && rawOutput.summary.trim().length > 0
    ? rawOutput.summary.trim()
    : normalized.length > 0
      ? `Found ${normalized.length} potential issue${normalized.length === 1 ? '' : 's'} that should be addressed.`
      : '✅ No issues found in the current changes.';

fs.writeFileSync('review-comments.json', JSON.stringify({ summary, comments: normalized }, null, 2));
fs.writeFileSync('skipped-comments.json', JSON.stringify(skipped, null, 2));
