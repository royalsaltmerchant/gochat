const IDENTITY_KEY = "parch.chat.identity";

function getCrypto() {
  const runtimeCrypto = globalThis?.crypto;
  if (!runtimeCrypto?.subtle) {
    throw new Error("WebCrypto API is not available");
  }
  return runtimeCrypto;
}

function toBase64Raw(bytes) {
  let binary = "";
  const arr = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  for (let i = 0; i < arr.length; i += 1) {
    binary += String.fromCharCode(arr[i]);
  }
  return btoa(binary).replace(/=+$/g, "");
}

function fromBase64Raw(str) {
  const padLength = (4 - (str.length % 4)) % 4;
  const padded = str + "=".repeat(padLength);
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function readIdentity() {
  try {
    const raw = localStorage.getItem(IDENTITY_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (!parsed.publicKey || !parsed.privateKey) return null;
    return parsed;
  } catch (_err) {
    return null;
  }
}

function writeIdentity(identity) {
  localStorage.setItem(IDENTITY_KEY, JSON.stringify(identity));
}

function normalizeIdentity(input) {
  if (!input || typeof input !== "object") {
    throw new Error("Identity payload must be an object");
  }

  const publicKey = typeof input.publicKey === "string" ? input.publicKey.trim() : "";
  const privateKey = typeof input.privateKey === "string" ? input.privateKey.trim() : "";
  const encPublicKey = typeof input.encPublicKey === "string" ? input.encPublicKey.trim() : "";
  const encPrivateKey = typeof input.encPrivateKey === "string" ? input.encPrivateKey.trim() : "";
  const username = typeof input.username === "string" ? input.username.trim() : "";

  if (!publicKey || !privateKey || !encPublicKey || !encPrivateKey) {
    throw new Error("Identity must include auth and encryption keypairs");
  }

  return { publicKey, privateKey, encPublicKey, encPrivateKey, username };
}

async function generateEncryptionIdentity() {
  const crypto = getCrypto();
  const keyPair = await crypto.subtle.generateKey(
    { name: "ECDH", namedCurve: "P-256" },
    true,
    ["deriveBits"]
  );

  const encPublicKeySpki = await crypto.subtle.exportKey("spki", keyPair.publicKey);
  const encPrivateKeyPkcs8 = await crypto.subtle.exportKey("pkcs8", keyPair.privateKey);

  return {
    encPublicKey: toBase64Raw(encPublicKeySpki),
    encPrivateKey: toBase64Raw(encPrivateKeyPkcs8),
  };
}

async function generateIdentity() {
  const crypto = getCrypto();

  const keyPair = await crypto.subtle.generateKey(
    { name: "Ed25519" },
    true,
    ["sign", "verify"]
  );

  const publicKeyRaw = await crypto.subtle.exportKey("raw", keyPair.publicKey);
  const privateKeyPkcs8 = await crypto.subtle.exportKey("pkcs8", keyPair.privateKey);
  const encIdentity = await generateEncryptionIdentity();

  const identity = {
    publicKey: toBase64Raw(publicKeyRaw),
    privateKey: toBase64Raw(privateKeyPkcs8),
    encPublicKey: encIdentity.encPublicKey,
    encPrivateKey: encIdentity.encPrivateKey,
    username: "",
  };

  writeIdentity(identity);
  return identity;
}

async function ensureIdentity() {
  const existing = readIdentity();
  if (existing) {
    if (!existing.encPublicKey || !existing.encPrivateKey) {
      const encIdentity = await generateEncryptionIdentity();
      const upgraded = {
        ...existing,
        encPublicKey: encIdentity.encPublicKey,
        encPrivateKey: encIdentity.encPrivateKey,
      };
      writeIdentity(upgraded);
      return upgraded;
    }
    return existing;
  }
  return generateIdentity();
}

async function signAuthMessage(identity, message) {
  const crypto = getCrypto();
  const privateKeyBytes = fromBase64Raw(identity.privateKey);
  const privateKey = await crypto.subtle.importKey(
    "pkcs8",
    privateKeyBytes,
    { name: "Ed25519" },
    false,
    ["sign"]
  );

  const payload = new TextEncoder().encode(message);
  const signature = await crypto.subtle.sign({ name: "Ed25519" }, privateKey, payload);
  return toBase64Raw(signature);
}

async function verifyAuthSignature(publicKey, message, signature) {
  const crypto = getCrypto();
  const publicKeyBytes = fromBase64Raw(publicKey);
  const signatureBytes = fromBase64Raw(signature);
  const importedPublicKey = await crypto.subtle.importKey(
    "raw",
    publicKeyBytes,
    { name: "Ed25519" },
    false,
    ["verify"]
  );
  const payload = new TextEncoder().encode(message);
  return crypto.subtle.verify({ name: "Ed25519" }, importedPublicKey, signatureBytes, payload);
}

async function validateIdentity(identity) {
  const crypto = getCrypto();
  const privateKeyBytes = fromBase64Raw(identity.privateKey);
  const publicKeyBytes = fromBase64Raw(identity.publicKey);
  const encPrivateKeyBytes = fromBase64Raw(identity.encPrivateKey);
  const encPublicKeyBytes = fromBase64Raw(identity.encPublicKey);

  const privateKey = await crypto.subtle.importKey(
    "pkcs8",
    privateKeyBytes,
    { name: "Ed25519" },
    false,
    ["sign"]
  );

  const publicKey = await crypto.subtle.importKey(
    "raw",
    publicKeyBytes,
    { name: "Ed25519" },
    false,
    ["verify"]
  );

  const message = new TextEncoder().encode("parch-identity-check");
  const signature = await crypto.subtle.sign({ name: "Ed25519" }, privateKey, message);
  const verified = await crypto.subtle.verify({ name: "Ed25519" }, publicKey, signature, message);

  if (!verified) {
    throw new Error("Public/private key mismatch");
  }

  await crypto.subtle.importKey(
    "pkcs8",
    encPrivateKeyBytes,
    { name: "ECDH", namedCurve: "P-256" },
    false,
    ["deriveBits"]
  );
  await crypto.subtle.importKey(
    "spki",
    encPublicKeyBytes,
    { name: "ECDH", namedCurve: "P-256" },
    false,
    []
  );
}

async function getOrCreateIdentity() {
  return ensureIdentity();
}

async function exportIdentity() {
  const identity = await ensureIdentity();
  return JSON.stringify(identity, null, 2);
}

async function importIdentity(serializedIdentity) {
  let parsed = serializedIdentity;

  if (typeof serializedIdentity === "string") {
    try {
      parsed = JSON.parse(serializedIdentity);
    } catch (_err) {
      throw new Error("Identity import text must be valid JSON");
    }
  }

  const normalized = normalizeIdentity(parsed);
  await validateIdentity(normalized);
  writeIdentity(normalized);
  return normalized;
}

function setIdentityUsername(username) {
  const identity = readIdentity();
  if (!identity) return;
  identity.username = (username || "").trim();
  writeIdentity(identity);
}

function clearIdentity() {
  localStorage.removeItem(IDENTITY_KEY);
}

export default {
  getOrCreateIdentity,
  signAuthMessage,
  verifyAuthSignature,
  exportIdentity,
  importIdentity,
  setIdentityUsername,
  clearIdentity,
};
