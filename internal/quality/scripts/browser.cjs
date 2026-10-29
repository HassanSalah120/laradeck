'use strict';
// Private local controller. Never emits page HTML, credentials or cookie data.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { createRequire } = require('node:module');
const config = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const runtimeRequire = createRequire(path.join(config.runtime, 'package.json'));
const { chromium } = runtimeRequire('playwright');
const emit = value => process.stdout.write(JSON.stringify(value) + '\n');
const displayURL = value => { const u = new URL(value); u.search = ''; u.hash = ''; return u.toString(); };
let context, server, initialNavigation = Promise.resolve(), busy = false, closing = false;
async function close() {
  if (closing) return;
  closing = true;
  try { if (context) await context.close(); } catch {}
  if (server) server.close();
  setTimeout(() => process.exit(0), 50).unref();
}
async function audit(page, address) {
  const requested = new URL(address);
  if (requested.origin !== config.origin || requested.username || requested.password)
    throw new Error('Select a page on the project origin');
  const response = await page.goto(requested.href, { waitUntil: 'domcontentloaded', timeout: 30000 });
  const final = new URL(page.url());
  const report = { status: 'completed', finalUrl: displayURL(final.href), findings: [], message: '' };
  if (final.origin !== config.origin || final.pathname !== requested.pathname) {
    report.status = 'skipped';
    report.message = 'Navigation redirected. Sign in manually, then select the actual target page explicitly.';
    return report;
  }
  if (!response || response.status() >= 400) {
    report.status = 'failed'; report.message = 'Page did not return a successful response'; return report;
  }
  await page.addScriptTag({ path: runtimeRequire.resolve('axe-core/axe.min.js') });
  report.findings = await page.evaluate(async () => {
    const results = await window.axe.run(document, { resultTypes: ['violations', 'incomplete'] });
    const map = (entry, certainty) => ({ rule: entry.id, message: entry.help, reference: entry.helpUrl,
      impact: entry.impact || 'minor', count: entry.nodes.length, certainty });
    return [...results.violations.map(x => map(x, 'detected')), ...results.incomplete.map(x => map(x, 'needs_review'))];
  });
  report.message = 'Rendered page checked. Automated accessibility checks do not establish complete WCAG compliance.';
  return report;
}
(async () => {
  context = await chromium.launchPersistentContext(config.profile, {
    headless: !!config.headless, acceptDownloads: false,
    ignoreHTTPSErrors: false,
  });
  const page = context.pages()[0] || await context.newPage();
  context.on('close', close);
  server = http.createServer(async (req, res) => {
    res.setHeader('Content-Type', 'application/json');
    const supplied = Buffer.from(req.headers['x-workbench-token'] || '');
    const expected = Buffer.from(config.token);
    if (req.method !== 'POST' || supplied.length !== expected.length || !crypto.timingSafeEqual(supplied, expected)) {
      res.writeHead(403); res.end('{}'); return;
    }
    if (req.url === '/close') { res.end('{}'); void close(); return; }
    if (req.url !== '/audit') { res.writeHead(404); res.end('{}'); return; }
    if (busy) { res.writeHead(409); res.end('{}'); return; }
    busy = true;
    try {
      let text = '';
      for await (const chunk of req) { text += chunk; if (text.length > 16384) throw new Error('Request too large'); }
      const input = JSON.parse(text);
      await initialNavigation;
      const result = await audit(page, input.url);
      res.end(JSON.stringify(result));
    } catch {
      res.writeHead(500); res.end(JSON.stringify({ status: 'failed', message: 'Browser check failed. Verify the selected page, local certificate and browser session.', findings: [] }));
    } finally { busy = false; }
  });
  server.requestTimeout = 45000;
  server.headersTimeout = 10000;
  server.listen(0, '127.0.0.1', () => {
    initialNavigation = page.goto(config.url, { waitUntil: 'domcontentloaded', timeout: 30000 }).catch(() => {});
    emit({ event: 'ready', port: server.address().port });
  });
})().catch(() => { emit({ event: 'error', message: 'Browser runtime failed to start. Recheck browser setup.' }); void close(); });
