import assert from "node:assert/strict";
import test from "node:test";
import { updatedAtInput, updatedAtTimestamp } from "../lib/article-dates.ts";

test("last updated input uses China time across browser timezones and day boundaries", () => {
  const original = process.env.TZ;
  try {
    for (const timezone of ["UTC", "America/New_York", "Asia/Shanghai"]) {
      process.env.TZ = timezone;
      assert.equal(updatedAtInput("2024-12-31T18:30:15.123Z"), "2025-01-01T02:30:15.123");
      assert.equal(updatedAtTimestamp("2025-01-01T02:30:15.123"), "2024-12-31T18:30:15.123Z");
      assert.equal(updatedAtTimestamp("2025-01-01T02:30"), "2024-12-31T18:30:00.000Z");
    }
  } finally {
    if (original === undefined) delete process.env.TZ;
    else process.env.TZ = original;
  }
});

test("an empty last updated time keeps automatic publication dating", () => {
  assert.equal(updatedAtInput(null), "");
  assert.equal(updatedAtInput(undefined), "");
  assert.equal(updatedAtTimestamp(""), null);
});
