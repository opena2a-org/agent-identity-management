import { expect, test } from '@playwright/test';

// Positive control for the E2E job. This one test fails on purpose, so a run of
// the job shows that a failing test turns the job red. It is never merged.
test('positive control: this assertion fails on purpose', () => {
  expect(1 + 1).toBe(3);
});
