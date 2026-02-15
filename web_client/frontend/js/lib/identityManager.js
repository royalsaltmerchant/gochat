const IDENTITY_KEY = "parch.chat.identity";

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
  const username = typeof input.username === "string" ? input.username.trim() : "";

  if (!publicKey || !privateKey) {
    throw new Error("Identity must include publicKey and privateKey");
  }

  return { publicKey, privateKey, username };
}

async function generateIdentity() {
  if (!window.crypto?.subtle) {
    throw new Error("WebCrypto API is not available");
  }

  const keyPair = await window.crypto.subtle.generateKey(
    { name: "Ed25519" },
    true,
    ["sign", "verify"]
  );

  const publicKeyRaw = await window.crypto.subtle.exportKey("raw", keyPair.publicKey);
  const privateKeyPkcs8 = await window.crypto.subtle.exportKey("pkcs8", keyPair.privateKey);

  const identity = {
    publicKey: toBase64Raw(publicKeyRaw),
    privateKey: toBase64Raw(privateKeyPkcs8),
    username: "",
  };

  writeIdentity(identity);
  return identity;
}

async function ensureIdentity() {
  const existing = readIdentity();
  if (existing) return existing;
  return generateIdentity();
}

async function signAuthMessage(identity, message) {
  const privateKeyBytes = fromBase64Raw(identity.privateKey);
  const privateKey = await window.crypto.subtle.importKey(
    "pkcs8",
    privateKeyBytes,
    { name: "Ed25519" },
    false,
    ["sign"]
  );

  const payload = new TextEncoder().encode(message);
  const signature = await window.crypto.subtle.sign({ name: "Ed25519" }, privateKey, payload);
  return toBase64Raw(signature);
}

async function validateIdentity(identity) {
  const privateKeyBytes = fromBase64Raw(identity.privateKey);
  const publicKeyBytes = fromBase64Raw(identity.publicKey);

  const privateKey = await window.crypto.subtle.importKey(
    "pkcs8",
    privateKeyBytes,
    { name: "Ed25519" },
    false,
    ["sign"]
  );

  const publicKey = await window.crypto.subtle.importKey(
    "raw",
    publicKeyBytes,
    { name: "Ed25519" },
    false,
    ["verify"]
  );

  const message = new TextEncoder().encode("parch-identity-check");
  const signature = await window.crypto.subtle.sign({ name: "Ed25519" }, privateKey, message);
  const verified = await window.crypto.subtle.verify({ name: "Ed25519" }, publicKey, signature, message);

  if (!verified) {
    throw new Error("Public/private key mismatch");
  }
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
  exportIdentity,
  importIdentity,
  setIdentityUsername,
  clearIdentity,
};
