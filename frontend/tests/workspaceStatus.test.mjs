import test from 'node:test';
import assert from 'node:assert/strict';
import { versionStatus, auditAgeStatus, coverageStatus } from '../src/workspaceStatus.ts';

test('versions compare release branches without assuming patch freshness', () => {
  for (const [kind, values] of [['Laravel', ['13.0.1', '12.25.0', '11.40.0']], ['PHP', ['8.5.0', '8.4.13', '8.3.27']]]) {
    assert.deepEqual(values.map(v => versionStatus(kind, v).tone), ['good', 'warn', 'danger']);
  }
  assert.equal(versionStatus('Laravel', 'v13.1.0').label, '13.1.0');
  for (const value of ['', 'unknown', '^13.0', '13.0.0-beta1', '14.0.0']) assert.equal(versionStatus('Laravel', value).tone, 'neutral');
  for (const value of ['8', '9.0.0', '8.6.0']) assert.equal(versionStatus('PHP', value).tone, 'neutral');
});

test('audit freshness handles boundaries and unavailable timestamps', () => {
  const now = Date.parse('2026-10-18T12:00:00Z');
  assert.equal(auditAgeStatus('2026-10-18T00:00:00Z', now).tone, 'good');
  assert.equal(auditAgeStatus('2026-10-17T12:00:00Z', now).tone, 'warn');
  assert.equal(auditAgeStatus('2026-10-11T12:00:00Z', now).tone, 'danger');
  for (const value of ['', 'invalid', '2027-01-01']) assert.equal(auditAgeStatus(value, now).tone, 'neutral');
});

test('coverage distinguishes complete, partial, failed, running and unknown', () => {
  assert.deepEqual(['3/3 completed', '1/3 completed', '0/3 completed', 'Running', 'Not run', '0/0 completed', '4/3 completed'].map(v => coverageStatus(v).tone), ['good', 'warn', 'danger', 'info', 'neutral', 'neutral', 'neutral']);
});
