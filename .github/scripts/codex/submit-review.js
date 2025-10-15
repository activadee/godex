const fs = require('fs');

module.exports = async ({ github, context, core }) => {
  const reviewPath = process.env.REVIEW_FILE;
  if (!reviewPath || !fs.existsSync(reviewPath)) {
    core.setFailed(`Missing review file: ${reviewPath}`);
    return;
  }

  const data = JSON.parse(fs.readFileSync(reviewPath, 'utf8'));
  const comments = Array.isArray(data.comments) ? data.comments : [];
  const summaryText = typeof data.summary === 'string' && data.summary.trim().length > 0
    ? data.summary.trim()
    : (comments.length
      ? `Found ${comments.length} potential issue${comments.length === 1 ? '' : 's'} that should be addressed.`
      : 'No blocking issues detected.');

  const prNumber = context.payload.pull_request.number;
  const headSha = context.payload.pull_request.head.sha;

  const normalizeBody = (body) => (typeof body === 'string' ? body.trim() : '');
  const commentKey = (path, line, body) =>
    `${path ?? ''}#${Number.isInteger(line) ? line : 'null'}#${normalizeBody(body)}`;
  const summarizeInlineBody = (body) => {
    const normalized = normalizeBody(body);
    const lines = normalized.split('\n').map((line) => line.trim()).filter(Boolean);
    const firstContent = lines.find((line) => !line.startsWith('```'));
    return firstContent ?? lines[0] ?? normalized;
  };

  const newCommentKeys = new Set();
  const createdComments = [];
  const failedComments = [];

  for (const comment of comments) {
    const path = comment.path;
    const line = comment.line;
    const body = comment.body;
    const key = commentKey(path, line, body);
    newCommentKeys.add(key);
    try {
      await github.rest.pulls.createReviewComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        pull_number: prNumber,
        commit_id: headSha,
        path,
        side: comment.side ?? 'RIGHT',
        line,
        body,
      });
      createdComments.push(comment);
    } catch (error) {
      core.warning(`Failed to create inline comment for ${path}:${line} – ${error.message}`);
      failedComments.push({ comment, error: error.message });
    }
  }

  let resolvedCount = 0;
  try {
    const threadsPath = 'existing_review_threads.json';
    if (fs.existsSync(threadsPath)) {
      const threads = JSON.parse(fs.readFileSync(threadsPath, 'utf8'));
      for (const thread of threads) {
        if (!thread || thread.resolved) continue;
        const threadComments = thread.comments ?? [];
        if (!threadComments.length) continue;
        const authoredByBot = threadComments.every((c) => (c.author ?? '').includes('[bot]'));
        if (!authoredByBot) continue;
        const threadKeys = threadComments.map((c) =>
          commentKey(c.path, Number.isInteger(c.line) ? c.line : null, c.body),
        );
        const stillRelevant = threadKeys.some((key) => newCommentKeys.has(key));
        if (stillRelevant) continue;
        await github.graphql(
          `mutation($threadId: ID!) {
            resolveReviewThread(input: { threadId: $threadId }) {
              thread { id isResolved }
            }
          }`,
          { threadId: thread.id },
        );
        resolvedCount += 1;
      }
    }
  } catch (error) {
    core.warning(`Failed to resolve prior threads: ${error.message}`);
  }

  const marker = '<!-- codex-review-summary -->';
  const recommendation = comments.length
    ? `🚫 Changes required – ${comments.length} inline issue${comments.length === 1 ? '' : 's'} detected.`
    : '✅ Ready to merge – no blocking issues detected.';

  const findingsSection = comments.length
    ? comments
        .map((c) => `- \`${c.path}:${c.line}\` — ${summarizeInlineBody(c.body)}`)
        .join('\n')
    : '- No inline findings.';

  const timestamp = new Date().toISOString();
  const summaryLines = [
    marker,
    '### Codex Review Summary',
    '',
    `Recommendation: ${recommendation}`,
    '',
    'Summary:',
    summaryText,
    '',
    'Inline findings:',
    findingsSection,
    '',
    resolvedCount
      ? `Resolved ${resolvedCount} previously open Codex thread${resolvedCount === 1 ? '' : 's'}.`
      : null,
    `Generated at ${timestamp}.`,
  ].filter((line) => line !== null);
  const summaryBody = summaryLines.join('\n');

  const existingSummary = await github.paginate(
    github.rest.issues.listComments,
    {
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: prNumber,
      per_page: 100,
    },
  ).then((comments) =>
    comments.find(
      (comment) =>
        (comment.user?.login || '').includes('[bot]') &&
        typeof comment.body === 'string' &&
        comment.body.includes(marker),
    ),
  );

  if (existingSummary) {
    await github.rest.issues.updateComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      comment_id: existingSummary.id,
      body: summaryBody,
    });
  } else {
    await github.rest.issues.createComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: prNumber,
      body: summaryBody,
    });
  }

  if (failedComments.length) {
    core.setFailed(
      `Failed to create ${failedComments.length} inline comment${failedComments.length === 1 ? '' : 's'}. Check workflow logs.`,
    );
    return;
  }

  core.info(
    `Posted ${createdComments.length} inline comment${createdComments.length === 1 ? '' : 's'} and updated summary comment (${resolvedCount} threads resolved).`,
  );
};
