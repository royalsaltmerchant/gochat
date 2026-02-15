import test from "node:test";
import assert from "node:assert/strict";
import { webcrypto } from "node:crypto";

if (!globalThis.crypto) {
  globalThis.crypto = webcrypto;
}
if (!globalThis.atob) {
  globalThis.atob = (value) => Buffer.from(value, "base64").toString("binary");
}
if (!globalThis.btoa) {
  globalThis.btoa = (value) => Buffer.from(value, "binary").toString("base64");
}
if (!globalThis.localStorage) {
  const storage = new Map();
  globalThis.localStorage = {
    getItem(key) {
      return storage.has(key) ? storage.get(key) : null;
    },
    setItem(key, value) {
      storage.set(key, String(value));
    },
    removeItem(key) {
      storage.delete(key);
    },
    clear() {
      storage.clear();
    },
  };
}

import identityManager from "./identityManager.js";
import e2ee from "./e2ee.js";

function fromBase64Raw(str) {
  const padLength = (4 - (str.length % 4)) % 4;
  return Buffer.from(str + "=".repeat(padLength), "base64");
}

function toBase64Raw(buffer) {
  return Buffer.from(buffer).toString("base64").replace(/=+$/g, "");
}

async function createIdentity() {
  identityManager.clearIdentity();
  return identityManager.getOrCreateIdentity();
}

test("encrypts and decrypts envelope across two identities", async () => {
  const alice = await createIdentity();
  const bob = await createIdentity();

  const envelope = await e2ee.encryptMessageForSpace({
    plaintext: "hello e2ee",
    spaceUUID: "space-1",
    channelUUID: "chan-1",
    identity: alice,
    recipients: [
      { authPublicKey: alice.publicKey, encPublicKey: alice.encPublicKey },
      { authPublicKey: bob.publicKey, encPublicKey: bob.encPublicKey },
    ],
  });

  const bobPlain = await e2ee.decryptMessageForIdentity({
    envelope,
    identity: bob,
  });
  const alicePlain = await e2ee.decryptMessageForIdentity({
    envelope,
    identity: alice,
  });

  assert.equal(bobPlain, "hello e2ee");
  assert.equal(alicePlain, "hello e2ee");
});

test("detects ciphertext tampering", async () => {
  const alice = await createIdentity();
  const bob = await createIdentity();

  const envelope = await e2ee.encryptMessageForSpace({
    plaintext: "integrity-check",
    spaceUUID: "space-2",
    channelUUID: "chan-2",
    identity: alice,
    recipients: [
      { authPublicKey: bob.publicKey, encPublicKey: bob.encPublicKey },
    ],
  });

  const tampered = structuredClone(envelope);
  tampered.ciphertext = tampered.ciphertext.slice(0, -1) + (tampered.ciphertext.endsWith("A") ? "B" : "A");

  await assert.rejects(
    () => e2ee.decryptMessageForIdentity({ envelope: tampered, identity: bob }),
    /Invalid message signature|OperationError|decrypt/i
  );
});

test("detects signature tampering", async () => {
  const alice = await createIdentity();
  const bob = await createIdentity();

  const envelope = await e2ee.encryptMessageForSpace({
    plaintext: "signed-envelope",
    spaceUUID: "space-3",
    channelUUID: "chan-3",
    identity: alice,
    recipients: [
      { authPublicKey: bob.publicKey, encPublicKey: bob.encPublicKey },
    ],
  });

  const tampered = structuredClone(envelope);
  const sigBytes = fromBase64Raw(tampered.sig);
  sigBytes[0] ^= 0x01;
  tampered.sig = toBase64Raw(sigBytes);

  await assert.rejects(
    () => e2ee.decryptMessageForIdentity({ envelope: tampered, identity: bob }),
    /Invalid message signature/i
  );
});
