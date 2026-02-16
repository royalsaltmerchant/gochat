const IDENTITY_KEY = "parch.chat.identity";
const IDENTITY_SEALED_KEY = "parch.chat.identity.sealed";
const DEVICE_ID_KEY = "parch.chat.device_id";
const LAST_EXPORTED_AT_KEY = "parch.chat.identity.last_exported_at";

const IDENTITY_DB_NAME = "parch.chat.identity.keys";
const IDENTITY_DB_VERSION = 1;
const IDENTITY_DB_STORE = "kv";
const DEVICE_WRAP_KEY_ID = "device_wrap_key";
const PASSPHRASE_ITERATIONS = 210000;

let cachedIdentity = null;
let cachedPrivateBundle = null;
let cachedSigningPrivateKey = null;
let cachedEncryptionPrivateKey = null;
let idbPromise = null;
let deviceWrapKeyPromise = null;

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

function supportsIndexedDB() {
  return typeof indexedDB !== "undefined";
}

function supportsSecureKeyVault() {
  return supportsIndexedDB();
}

function readJSONStorage(key) {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object") return null;
    return parsed;
  } catch (_err) {
    return null;
  }
}

function writeJSONStorage(key, value) {
  localStorage.setItem(key, JSON.stringify(value));
}

function clearIdentityCache() {
  cachedIdentity = null;
  cachedPrivateBundle = null;
  cachedSigningPrivateKey = null;
  cachedEncryptionPrivateKey = null;
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

function normalizeIdentityMeta(input) {
  if (!input || typeof input !== "object") return null;
  const publicKey = typeof input.publicKey === "string" ? input.publicKey.trim() : "";
  const encPublicKey = typeof input.encPublicKey === "string" ? input.encPublicKey.trim() : "";
  const username = typeof input.username === "string" ? input.username.trim() : "";
  if (!publicKey || !encPublicKey) return null;
  return {
    version: 2,
    publicKey,
    encPublicKey,
    username,
  };
}

function normalizePrivateBundle(input) {
  if (!input || typeof input !== "object") {
    throw new Error("Invalid private key bundle");
  }
  const privateKey = typeof input.privateKey === "string" ? input.privateKey.trim() : "";
  const encPrivateKey = typeof input.encPrivateKey === "string" ? input.encPrivateKey.trim() : "";
  if (!privateKey || !encPrivateKey) {
    throw new Error("Private key bundle is incomplete");
  }
  return { privateKey, encPrivateKey };
}

function getOrCreateDeviceID() {
  const existing = localStorage.getItem(DEVICE_ID_KEY);
  if (typeof existing === "string" && existing.trim() !== "") {
    return existing.trim();
  }
  const generated = toBase64Raw(getCrypto().getRandomValues(new Uint8Array(16)));
  localStorage.setItem(DEVICE_ID_KEY, generated);
  return generated;
}

function detectBrowserName() {
  const ua = (globalThis?.navigator?.userAgent || "").toLowerCase();
  if (ua.includes("edg/")) return "Edge";
  if (ua.includes("firefox/")) return "Firefox";
  if (ua.includes("safari/") && !ua.includes("chrome/")) return "Safari";
  if (ua.includes("chrome/")) return "Chrome";
  return "Browser";
}

function getDeviceMetadata() {
  const deviceId = getOrCreateDeviceID();
  const browser = detectBrowserName();
  const platform = String(globalThis?.navigator?.platform || "Unknown");
  const deviceName = `${browser} on ${platform}`.slice(0, 80);
  return { deviceId, deviceName };
}

function markIdentityExportedNow() {
  localStorage.setItem(LAST_EXPORTED_AT_KEY, new Date().toISOString());
}

function getLastExportedAt() {
  const value = localStorage.getItem(LAST_EXPORTED_AT_KEY);
  return typeof value === "string" ? value : "";
}

function requirePassphrase(passphrase) {
  const value = typeof passphrase === "string" ? passphrase : "";
  if (value.length < 8) {
    throw new Error("Passphrase must be at least 8 characters");
  }
  return value;
}

async function importAuthPrivateKey(privateKeyB64) {
  const crypto = getCrypto();
  return crypto.subtle.importKey(
    "pkcs8",
    fromBase64Raw(privateKeyB64),
    { name: "Ed25519" },
    false,
    ["sign"]
  );
}

async function importEncPrivateKey(encPrivateKeyB64) {
  const crypto = getCrypto();
  return crypto.subtle.importKey(
    "pkcs8",
    fromBase64Raw(encPrivateKeyB64),
    { name: "ECDH", namedCurve: "P-256" },
    false,
    ["deriveBits"]
  );
}

async function importAuthPublicKey(publicKeyB64) {
  const crypto = getCrypto();
  return crypto.subtle.importKey(
    "raw",
    fromBase64Raw(publicKeyB64),
    { name: "Ed25519" },
    false,
    ["verify"]
  );
}

async function importEncPublicKey(encPublicKeyB64) {
  const crypto = getCrypto();
  return crypto.subtle.importKey(
    "spki",
    fromBase64Raw(encPublicKeyB64),
    { name: "ECDH", namedCurve: "P-256" },
    false,
    []
  );
}

async function validateIdentity(identity) {
  const privateKey = await importAuthPrivateKey(identity.privateKey);
  const publicKey = await importAuthPublicKey(identity.publicKey);

  const message = new TextEncoder().encode("parch-identity-check");
  const signature = await getCrypto().subtle.sign({ name: "Ed25519" }, privateKey, message);
  const verified = await getCrypto().subtle.verify({ name: "Ed25519" }, publicKey, signature, message);

  if (!verified) {
    throw new Error("Public/private key mismatch");
  }

  await importEncPrivateKey(identity.encPrivateKey);
  await importEncPublicKey(identity.encPublicKey);
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

async function openIdentityDB() {
  if (!supportsIndexedDB()) {
    return null;
  }
  if (idbPromise) {
    return idbPromise;
  }

  idbPromise = new Promise((resolve) => {
    try {
      const request = indexedDB.open(IDENTITY_DB_NAME, IDENTITY_DB_VERSION);
      request.onerror = () => resolve(null);
      request.onupgradeneeded = () => {
        const db = request.result;
        if (!db.objectStoreNames.contains(IDENTITY_DB_STORE)) {
          db.createObjectStore(IDENTITY_DB_STORE);
        }
      };
      request.onsuccess = () => resolve(request.result);
    } catch (_err) {
      resolve(null);
    }
  });

  return idbPromise;
}

async function idbGet(db, key) {
  return new Promise((resolve) => {
    try {
      const tx = db.transaction(IDENTITY_DB_STORE, "readonly");
      const store = tx.objectStore(IDENTITY_DB_STORE);
      const req = store.get(key);
      req.onsuccess = () => resolve(req.result || null);
      req.onerror = () => resolve(null);
    } catch (_err) {
      resolve(null);
    }
  });
}

async function idbPut(db, key, value) {
  return new Promise((resolve) => {
    try {
      const tx = db.transaction(IDENTITY_DB_STORE, "readwrite");
      tx.oncomplete = () => resolve(true);
      tx.onerror = () => resolve(false);
      tx.objectStore(IDENTITY_DB_STORE).put(value, key);
    } catch (_err) {
      resolve(false);
    }
  });
}

async function idbDelete(db, key) {
  return new Promise((resolve) => {
    try {
      const tx = db.transaction(IDENTITY_DB_STORE, "readwrite");
      tx.oncomplete = () => resolve(true);
      tx.onerror = () => resolve(false);
      tx.objectStore(IDENTITY_DB_STORE).delete(key);
    } catch (_err) {
      resolve(false);
    }
  });
}

async function getOrCreateDeviceWrapKey() {
  if (!supportsSecureKeyVault()) {
    return null;
  }
  if (deviceWrapKeyPromise) {
    return deviceWrapKeyPromise;
  }

  deviceWrapKeyPromise = (async () => {
    const db = await openIdentityDB();
    if (!db) {
      return null;
    }
    const existing = await idbGet(db, DEVICE_WRAP_KEY_ID);
    if (existing) {
      return existing;
    }
    const key = await getCrypto().subtle.generateKey(
      { name: "AES-GCM", length: 256 },
      false,
      ["encrypt", "decrypt"]
    );
    const stored = await idbPut(db, DEVICE_WRAP_KEY_ID, key);
    return stored ? key : null;
  })().catch(() => null);

  return deviceWrapKeyPromise;
}

async function derivePassphraseKey(passphrase, salt, usages) {
  const passphraseBytes = new TextEncoder().encode(passphrase);
  const baseKey = await getCrypto().subtle.importKey("raw", passphraseBytes, "PBKDF2", false, ["deriveKey"]);
  return getCrypto().subtle.deriveKey(
    {
      name: "PBKDF2",
      hash: "SHA-256",
      iterations: PASSPHRASE_ITERATIONS,
      salt,
    },
    baseKey,
    { name: "AES-GCM", length: 256 },
    false,
    usages
  );
}

async function encryptBytesWithPassphrase(plainBytes, passphrase) {
  const salt = getCrypto().getRandomValues(new Uint8Array(16));
  const iv = getCrypto().getRandomValues(new Uint8Array(12));
  const key = await derivePassphraseKey(passphrase, salt, ["encrypt"]);
  const ciphertext = await getCrypto().subtle.encrypt({ name: "AES-GCM", iv }, key, plainBytes);
  return {
    type: "parch.encrypted_identity_backup.v1",
    kdf: {
      name: "PBKDF2",
      hash: "SHA-256",
      iterations: PASSPHRASE_ITERATIONS,
      salt: toBase64Raw(salt),
    },
    cipher: {
      name: "AES-GCM",
      iv: toBase64Raw(iv),
      ciphertext: toBase64Raw(ciphertext),
    },
  };
}

async function decryptBytesWithPassphrase(payload, passphrase) {
  if (!payload || payload.type !== "parch.encrypted_identity_backup.v1") {
    throw new Error("Unsupported backup format");
  }
  const iterations = Number(payload?.kdf?.iterations);
  if (!Number.isInteger(iterations) || iterations < 100000) {
    throw new Error("Invalid backup key derivation settings");
  }
  const salt = fromBase64Raw(String(payload?.kdf?.salt || ""));
  const iv = fromBase64Raw(String(payload?.cipher?.iv || ""));
  const ciphertext = fromBase64Raw(String(payload?.cipher?.ciphertext || ""));

  const passphraseBytes = new TextEncoder().encode(passphrase);
  const baseKey = await getCrypto().subtle.importKey("raw", passphraseBytes, "PBKDF2", false, ["deriveKey"]);
  const key = await getCrypto().subtle.deriveKey(
    {
      name: "PBKDF2",
      hash: "SHA-256",
      iterations,
      salt,
    },
    baseKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["decrypt"]
  );
  return getCrypto().subtle.decrypt({ name: "AES-GCM", iv }, key, ciphertext);
}

async function sealPrivateBundle(privateBundle) {
  const deviceWrapKey = await getOrCreateDeviceWrapKey();
  if (!deviceWrapKey) {
    return null;
  }
  const iv = getCrypto().getRandomValues(new Uint8Array(12));
  const plainBytes = new TextEncoder().encode(JSON.stringify(privateBundle));
  const ciphertext = await getCrypto().subtle.encrypt({ name: "AES-GCM", iv }, deviceWrapKey, plainBytes);
  return {
    type: "parch.identity.sealed.v1",
    iv: toBase64Raw(iv),
    ciphertext: toBase64Raw(ciphertext),
  };
}

async function unsealPrivateBundle(sealedRecord) {
  if (!sealedRecord || sealedRecord.type !== "parch.identity.sealed.v1") {
    throw new Error("Missing sealed private key material");
  }
  const deviceWrapKey = await getOrCreateDeviceWrapKey();
  if (!deviceWrapKey) {
    throw new Error("Secure key vault unavailable");
  }
  const iv = fromBase64Raw(String(sealedRecord.iv || ""));
  const ciphertext = fromBase64Raw(String(sealedRecord.ciphertext || ""));
  const plaintext = await getCrypto().subtle.decrypt({ name: "AES-GCM", iv }, deviceWrapKey, ciphertext);
  const parsed = JSON.parse(new TextDecoder().decode(plaintext));
  return normalizePrivateBundle(parsed);
}

function toPublicIdentity(identity) {
  const out = {
    publicKey: identity.publicKey,
    encPublicKey: identity.encPublicKey,
    username: identity.username || "",
  };
  if (!supportsSecureKeyVault()) {
    out.privateKey = identity.privateKey;
    out.encPrivateKey = identity.encPrivateKey;
  }
  return out;
}

async function cacheIdentityMaterial(identity) {
  cachedIdentity = toPublicIdentity(identity);
  cachedPrivateBundle = {
    privateKey: identity.privateKey,
    encPrivateKey: identity.encPrivateKey,
  };
  cachedSigningPrivateKey = await importAuthPrivateKey(identity.privateKey);
  cachedEncryptionPrivateKey = await importEncPrivateKey(identity.encPrivateKey);
}

async function persistIdentity(identity) {
  const normalized = normalizeIdentity(identity);
  await validateIdentity(normalized);

  const meta = {
    version: 2,
    publicKey: normalized.publicKey,
    encPublicKey: normalized.encPublicKey,
    username: normalized.username || "",
  };

  const sealed = await sealPrivateBundle({
    privateKey: normalized.privateKey,
    encPrivateKey: normalized.encPrivateKey,
  });

  if (sealed) {
    writeJSONStorage(IDENTITY_KEY, meta);
    writeJSONStorage(IDENTITY_SEALED_KEY, sealed);
  } else {
    // Fallback for runtimes without IndexedDB (e.g. some tests/environments).
    writeJSONStorage(IDENTITY_KEY, normalized);
    localStorage.removeItem(IDENTITY_SEALED_KEY);
  }

  await cacheIdentityMaterial(normalized);
  return cachedIdentity;
}

async function generateIdentity() {
  const crypto = getCrypto();
  const authKeyPair = await crypto.subtle.generateKey(
    { name: "Ed25519" },
    true,
    ["sign", "verify"]
  );

  const publicKeyRaw = await crypto.subtle.exportKey("raw", authKeyPair.publicKey);
  const privateKeyPkcs8 = await crypto.subtle.exportKey("pkcs8", authKeyPair.privateKey);
  const encIdentity = await generateEncryptionIdentity();

  return persistIdentity({
    publicKey: toBase64Raw(publicKeyRaw),
    privateKey: toBase64Raw(privateKeyPkcs8),
    encPublicKey: encIdentity.encPublicKey,
    encPrivateKey: encIdentity.encPrivateKey,
    username: "",
  });
}

async function ensureIdentityLoaded() {
  if (cachedIdentity && cachedSigningPrivateKey && cachedEncryptionPrivateKey && cachedPrivateBundle) {
    return cachedIdentity;
  }

  const stored = readJSONStorage(IDENTITY_KEY);
  if (!stored) {
    return generateIdentity();
  }

  // Legacy format migration path: private keys were stored directly in localStorage.
  if (typeof stored.privateKey === "string" && typeof stored.encPrivateKey === "string") {
    const normalizedLegacy = normalizeIdentity(stored);
    return persistIdentity(normalizedLegacy);
  }

  const meta = normalizeIdentityMeta(stored);
  if (!meta) {
    return generateIdentity();
  }

  const sealed = readJSONStorage(IDENTITY_SEALED_KEY);
  if (!sealed) {
    throw new Error("Identity private keys are unavailable on this device. Import an encrypted backup or reset identity.");
  }

  const privateBundle = await unsealPrivateBundle(sealed);
  const restoredIdentity = normalizeIdentity({
    publicKey: meta.publicKey,
    privateKey: privateBundle.privateKey,
    encPublicKey: meta.encPublicKey,
    encPrivateKey: privateBundle.encPrivateKey,
    username: meta.username || "",
  });

  await validateIdentity(restoredIdentity);
  await cacheIdentityMaterial(restoredIdentity);
  return cachedIdentity;
}

async function getSigningPrivateKey(_identity) {
  if (_identity?.privateKey) {
    return importAuthPrivateKey(_identity.privateKey);
  }
  await ensureIdentityLoaded();
  if (!cachedSigningPrivateKey) {
    throw new Error("Signing key unavailable");
  }
  return cachedSigningPrivateKey;
}

async function getEncryptionPrivateKey(_identity) {
  if (_identity?.encPrivateKey) {
    return importEncPrivateKey(_identity.encPrivateKey);
  }
  await ensureIdentityLoaded();
  if (!cachedEncryptionPrivateKey) {
    throw new Error("Encryption key unavailable");
  }
  return cachedEncryptionPrivateKey;
}

async function getOrCreateIdentity() {
  return ensureIdentityLoaded();
}

async function signAuthMessage(identity, message) {
  if (!message || typeof message !== "string") {
    throw new Error("Missing auth message");
  }
  const payload = new TextEncoder().encode(message);
  const privateKey = await getSigningPrivateKey(identity);
  const signature = await getCrypto().subtle.sign({ name: "Ed25519" }, privateKey, payload);
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

async function exportIdentity() {
  await ensureIdentityLoaded();
  if (!cachedIdentity || !cachedPrivateBundle) {
    throw new Error("Identity not available");
  }
  return JSON.stringify(
    {
      ...cachedIdentity,
      ...cachedPrivateBundle,
    },
    null,
    2
  );
}

async function exportEncryptedIdentity(passphrase) {
  const normalizedPassphrase = requirePassphrase(passphrase);
  await ensureIdentityLoaded();
  if (!cachedIdentity || !cachedPrivateBundle) {
    throw new Error("Identity not available");
  }
  const payloadBytes = new TextEncoder().encode(
    JSON.stringify({
      ...cachedIdentity,
      ...cachedPrivateBundle,
    })
  );
  const encrypted = await encryptBytesWithPassphrase(payloadBytes, normalizedPassphrase);
  return JSON.stringify(encrypted, null, 2);
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

  if (parsed?.type === "parch.encrypted_identity_backup.v1") {
    throw new Error("Encrypted backup detected. Use passphrase import.");
  }

  const normalized = normalizeIdentity(parsed);
  return persistIdentity(normalized);
}

async function importEncryptedIdentity(serializedBackup, passphrase) {
  const normalizedPassphrase = requirePassphrase(passphrase);

  let parsed = serializedBackup;
  if (typeof serializedBackup === "string") {
    try {
      parsed = JSON.parse(serializedBackup);
    } catch (_err) {
      throw new Error("Backup import text must be valid JSON");
    }
  }

  const decrypted = await decryptBytesWithPassphrase(parsed, normalizedPassphrase);
  let identityPayload;
  try {
    identityPayload = JSON.parse(new TextDecoder().decode(decrypted));
  } catch (_err) {
    throw new Error("Invalid backup payload");
  }

  const normalized = normalizeIdentity(identityPayload);
  return persistIdentity(normalized);
}

function setIdentityUsername(username) {
  const nextUsername = (username || "").trim();

  const stored = readJSONStorage(IDENTITY_KEY);
  if (stored && typeof stored.privateKey === "string" && typeof stored.encPrivateKey === "string") {
    stored.username = nextUsername;
    writeJSONStorage(IDENTITY_KEY, stored);
  } else if (stored && typeof stored === "object") {
    const meta = normalizeIdentityMeta(stored);
    if (meta) {
      meta.username = nextUsername;
      writeJSONStorage(IDENTITY_KEY, meta);
    }
  }

  if (cachedIdentity) {
    cachedIdentity.username = nextUsername;
  }
}

function clearIdentity() {
  clearIdentityCache();
  localStorage.removeItem(IDENTITY_KEY);
  localStorage.removeItem(IDENTITY_SEALED_KEY);

  openIdentityDB().then((db) => {
    if (db) {
      return idbDelete(db, DEVICE_WRAP_KEY_ID);
    }
    return false;
  }).catch(() => false);

  deviceWrapKeyPromise = null;
}

export default {
  getOrCreateIdentity,
  getDeviceMetadata,
  getSigningPrivateKey,
  getEncryptionPrivateKey,
  signAuthMessage,
  verifyAuthSignature,
  exportIdentity,
  exportEncryptedIdentity,
  markIdentityExportedNow,
  getLastExportedAt,
  importIdentity,
  importEncryptedIdentity,
  setIdentityUsername,
  clearIdentity,
};
