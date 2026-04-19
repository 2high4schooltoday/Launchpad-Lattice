"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { LatticeError, parse, stringify } = require("./index");

test("parses simple payload document", () => {
  const value = parse(`
lattice 1.

data:
  appId is "hello".
  retries is 3.
  enabled is true.
  architectures is list:
    item is "linux-x86_64".
    item is "linux-arm64".
  extension is record:
    bundle is uri "file:ext/demo.lext".
    sha256 is digest "sha256:abc".
`);

  assert.equal(value.appId, "hello");
  assert.equal(value.retries, 3);
  assert.equal(value.enabled, true);
  assert.equal(value.architectures[0], "linux-x86_64");
  assert.equal(value.extension.bundle, "file:ext/demo.lext");
});

test("ignores non-data sections when data is present", () => {
  const value = parse(`
lattice 1.

document is "org.example.demo".
schema is "org.example/demo/1".

data is Defaults:
  installDir is path "/opt/demo".
`);

  assert.equal(value.installDir, "/opt/demo");
});

test("renders round trip payloads", () => {
  const original = {
    appId: "hello",
    enabled: true,
    threshold: 2.5,
    labels: {
      "io.launchpad.app/name": "hello",
    },
    architectures: ["linux-x86_64", "linux-arm64"],
  };

  assert.deepEqual(parse(stringify(original)), original);
});

test("rejects duplicate keys", () => {
  assert.throws(
    () =>
      parse(`
lattice 1.

data:
  appId is "hello".
  appId is "duplicate".
`),
    (error) => {
      assert.ok(error instanceof LatticeError);
      assert.match(error.message, /duplicate key `appId`/u);
      return true;
    },
  );
});
