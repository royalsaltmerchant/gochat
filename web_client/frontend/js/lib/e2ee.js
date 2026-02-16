import identityManager from "./identityManager.js";

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

function canonicalEnvelopeForSignature(envelope) {
  const wrappedKeys = Array.isArray(envelope.wrapped_keys)
    ? [...envelope.wrapped_keys].sort((a, b) =>
        String(a.recipient_auth_public_key).localeCompare(String(b.recipient_auth_public_key))
      )
    : [];

  const canonical = {
    v: envelope.v,
    alg: envelope.alg,
    message_id: envelope.message_id,
    sender_timestamp: envelope.sender_timestamp,
    space_uuid: envelope.space_uuid,
    channel_uuid: envelope.channel_uuid,
    sender_auth_public_key: envelope.sender_auth_public_key,
    sender_enc_public_key: envelope.sender_enc_public_key,
    content_iv: envelope.content_iv,
    ciphertext: envelope.ciphertext,
    wrapped_keys: wrappedKeys.map((item) => ({
      recipient_auth_public_key: item.recipient_auth_public_key,
      iv: item.iv,
      ciphertext: item.ciphertext,
    })),
  };

  return JSON.stringify(canonical);
}

function createMessageID() {
  const crypto = getCrypto();
  return toBase64Raw(crypto.getRandomValues(new Uint8Array(16)));
}

async function importRecipientEncPublicKey(encodedKey) {
  const crypto = getCrypto();
  return crypto.subtle.importKey(
    "spki",
    fromBase64Raw(encodedKey),
    { name: "ECDH", namedCurve: "P-256" },
    false,
    []
  );
}

async function importSenderEncPublicKey(encodedKey) {
  return importRecipientEncPublicKey(encodedKey);
}

async function importOwnEncPrivateKey(encodedKey) {
  const crypto = getCrypto();
  return crypto.subtle.importKey(
    "pkcs8",
    fromBase64Raw(encodedKey),
    { name: "ECDH", namedCurve: "P-256" },
    false,
    ["deriveBits"]
  );
}

async function deriveWrapKey(privateKey, publicKey, envelopeContext) {
  const crypto = getCrypto();
  const sharedSecret = await crypto.subtle.deriveBits(
    { name: "ECDH", public: publicKey },
    privateKey,
    256
  );
  const hkdfBase = await crypto.subtle.importKey("raw", sharedSecret, "HKDF", false, [
    "deriveKey",
  ]);
  const salt = new TextEncoder().encode(
    `parch-e2ee-wrap-salt:${envelopeContext.space_uuid}:${envelopeContext.channel_uuid}`
  );
  const info = new TextEncoder().encode(
    `parch-e2ee-wrap-info:${envelopeContext.sender_auth_public_key}`
  );

  return crypto.subtle.deriveKey(
    {
      name: "HKDF",
      hash: "SHA-256",
      salt,
      info,
    },
    hkdfBase,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"]
  );
}

async function encryptMessageForSpace(params) {
  const crypto = getCrypto();
  const {
    plaintext,
    spaceUUID,
    channelUUID,
    identity,
    recipients,
  } = params;

  if (!identity?.publicKey || !identity?.privateKey || !identity?.encPublicKey || !identity?.encPrivateKey) {
    throw new Error("Missing sender identity keys");
  }
  if (!spaceUUID || !channelUUID) {
    throw new Error("Missing space/channel context");
  }
  if (!Array.isArray(recipients) || recipients.length === 0) {
    throw new Error("No recipient keys available for encryption");
  }

  const uniqueRecipients = [];
  const seenAuthKeys = new Set();
  for (const recipient of recipients) {
    if (!recipient?.authPublicKey || !recipient?.encPublicKey) continue;
    if (seenAuthKeys.has(recipient.authPublicKey)) continue;
    seenAuthKeys.add(recipient.authPublicKey);
    uniqueRecipients.push(recipient);
  }

  if (!seenAuthKeys.has(identity.publicKey)) {
    uniqueRecipients.push({
      authPublicKey: identity.publicKey,
      encPublicKey: identity.encPublicKey,
    });
  }

  const messageKey = await crypto.subtle.generateKey(
    { name: "AES-GCM", length: 256 },
    true,
    ["encrypt", "decrypt"]
  );
  const rawMessageKey = new Uint8Array(await crypto.subtle.exportKey("raw", messageKey));
  const contentIV = crypto.getRandomValues(new Uint8Array(12));
  const contentPlain = new TextEncoder().encode(plaintext);
  const contentCipher = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: contentIV },
    messageKey,
    contentPlain
  );

  const senderEncPrivate = await importOwnEncPrivateKey(identity.encPrivateKey);

  const envelope = {
    v: 1,
    alg: "p256-hkdf-aesgcm+ed25519",
    message_id: createMessageID(),
    sender_timestamp: new Date().toISOString(),
    space_uuid: spaceUUID,
    channel_uuid: channelUUID,
    sender_auth_public_key: identity.publicKey,
    sender_enc_public_key: identity.encPublicKey,
    content_iv: toBase64Raw(contentIV),
    ciphertext: toBase64Raw(contentCipher),
    wrapped_keys: [],
  };

  for (const recipient of uniqueRecipients) {
    const recipientPublic = await importRecipientEncPublicKey(recipient.encPublicKey);
    const wrapKey = await deriveWrapKey(senderEncPrivate, recipientPublic, envelope);
    const wrapIV = crypto.getRandomValues(new Uint8Array(12));
    const wrapped = await crypto.subtle.encrypt(
      { name: "AES-GCM", iv: wrapIV },
      wrapKey,
      rawMessageKey
    );
    envelope.wrapped_keys.push({
      recipient_auth_public_key: recipient.authPublicKey,
      iv: toBase64Raw(wrapIV),
      ciphertext: toBase64Raw(wrapped),
    });
  }

  const canonical = canonicalEnvelopeForSignature(envelope);
  envelope.sig = await identityManager.signAuthMessage(identity, canonical);

  return envelope;
}

async function decryptMessageForIdentity(params) {
  const crypto = getCrypto();
  const { envelope, identity } = params;

  if (!envelope?.sender_auth_public_key || !envelope?.sender_enc_public_key || !envelope?.sig) {
    throw new Error("Invalid encrypted envelope");
  }
  if (!identity?.publicKey || !identity?.encPrivateKey) {
    throw new Error("Missing recipient identity");
  }

  const canonical = canonicalEnvelopeForSignature(envelope);
  const signatureValid = await identityManager.verifyAuthSignature(
    envelope.sender_auth_public_key,
    canonical,
    envelope.sig
  );
  if (!signatureValid) {
    throw new Error("Invalid message signature");
  }

  const wrapped = (envelope.wrapped_keys || []).find(
    (item) => item.recipient_auth_public_key === identity.publicKey
  );
  if (!wrapped) {
    throw new Error("No wrapped key for this identity");
  }

  const recipientPrivate = await importOwnEncPrivateKey(identity.encPrivateKey);
  const senderPublic = await importSenderEncPublicKey(envelope.sender_enc_public_key);
  const wrapKey = await deriveWrapKey(recipientPrivate, senderPublic, envelope);

  const rawMessageKey = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: fromBase64Raw(wrapped.iv) },
    wrapKey,
    fromBase64Raw(wrapped.ciphertext)
  );
  const messageKey = await crypto.subtle.importKey(
    "raw",
    rawMessageKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["decrypt"]
  );

  const plaintextBuffer = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: fromBase64Raw(envelope.content_iv) },
    messageKey,
    fromBase64Raw(envelope.ciphertext)
  );

  return new TextDecoder().decode(plaintextBuffer);
}

export default {
  encryptMessageForSpace,
  decryptMessageForIdentity,
  canonicalEnvelopeForSignature,
};
