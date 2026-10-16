/**
 * The one decoder from an agent private key to the 32-byte seed Ed25519 signs
 * with. Every SDK path that signs with an agent private key goes through it.
 *
 * AIM issues an agent private key as standard padded base64 of 64 bytes: the
 * seed, then its public key. The 32-byte seed alone is accepted as well. Hex
 * is not: no AIM surface issues it.
 */

import { createPrivateKey, createPublicKey } from 'crypto';
import { ConfigurationError } from '../exceptions';

/** The Fix every refusal of an agent private key carries; it holds no part of any key. */
export const PRIVATE_KEY_FIX =
  'Set the agent private key to the standard padded base64 of its 32-byte seed, or of the 64 bytes AIM ' +
  'issues (the seed, then its public key), exactly as `registerAgent` returns it: convert a hex key with ' +
  "`Buffer.from(key, 'hex').toString('base64')`, and an unpadded, URL-safe or line-wrapped base64 key " +
  "with `Buffer.from(key, 'base64').toString('base64')`.";

const NOTHING_SIGNED = 'nothing was signed';

/** Standard base64 with correct padding; canonical form is checked separately. */
const STANDARD_BASE64 = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;

/** PKCS#8 DER header of an Ed25519 private key; the 32-byte seed follows it. */
const ED25519_PKCS8_HEADER = Buffer.from('302e020100300506032b657004220420', 'hex');

/**
 * The refusal of an agent private key. Its message is the case, then what was
 * not done, then `Fix: ` and `fix`; no part of the key is in it.
 */
export class PrivateKeyError extends ConfigurationError {
  /** The one Fix sentence, without the `Fix: ` label. */
  readonly fix: string;

  constructor(line: string, stem: string = NOTHING_SIGNED) {
    super(`${line}; ${stem}\nFix: ${PRIVATE_KEY_FIX}`);
    this.name = 'PrivateKeyError';
    this.fix = PRIVATE_KEY_FIX;
  }
}

function decodeStandardBase64(encoded: string): Uint8Array | null {
  if (!STANDARD_BASE64.test(encoded)) return null;
  const bytes = Buffer.from(encoded, 'base64');
  // Rejects non-zero padding bits, which decode to the same bytes as another
  // string: only the one canonical encoding of the key is accepted.
  return bytes.toString('base64') === encoded ? new Uint8Array(bytes) : null;
}

function bytesEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a[i] ^ b[i];
  return diff === 0;
}

/** The Ed25519 public key of a 32-byte seed. */
function publicKeyOf(seed: Uint8Array): Uint8Array {
  const privateKey = createPrivateKey({
    key: Buffer.concat([ED25519_PKCS8_HEADER, seed]),
    format: 'der',
    type: 'pkcs8',
  });
  const spki = createPublicKey(privateKey).export({ format: 'der', type: 'spki' });
  return new Uint8Array(spki.subarray(spki.length - 32));
}

/**
 * Decodes an agent private key to its 32-byte seed and the public key derived
 * from that seed. `normalizePrivateKey` documents what is accepted; `stem`
 * says in a refusal what was not done.
 *
 * @internal
 */
export function decodePrivateKey(
  input: string | Uint8Array,
  stem: string = NOTHING_SIGNED
): { seed: Uint8Array; publicKey: Uint8Array } {
  let bytes: Uint8Array;
  if (input instanceof Uint8Array) {
    bytes = input;
  } else if (typeof input === 'string' && input.length > 0) {
    const decoded = decodeStandardBase64(input);
    if (decoded === null) {
      throw new PrivateKeyError('the agent private key is not standard base64', stem);
    }
    bytes = decoded;
  } else {
    throw new PrivateKeyError('no agent private key is configured', stem);
  }
  if (bytes.length !== 32 && bytes.length !== 64) {
    throw new PrivateKeyError(`the agent private key is ${bytes.length} bytes long, not 32 or 64`, stem);
  }
  const seed = bytes.slice(0, 32);
  const publicKey = publicKeyOf(seed);
  if (bytes.length === 64 && !bytesEqual(publicKey, bytes.subarray(32))) {
    throw new PrivateKeyError(
      'the agent private key is 64 bytes long, and its last 32 bytes are not its public key',
      stem
    );
  }
  return { seed, publicKey };
}

/**
 * The 32-byte Ed25519 seed of an agent private key.
 *
 * Accepts the key as 32 bytes (the seed) or as the 64 bytes AIM issues (the
 * seed, then its public key), given as bytes or as standard padded base64 that
 * re-encodes to itself. In the 64-byte form the last 32 bytes must be the
 * public key derived from the first 32. Anything else, hex included, throws
 * `PrivateKeyError` with the Fix that converts it. Returns a copy.
 */
export function normalizePrivateKey(input: string | Uint8Array, stem: string = NOTHING_SIGNED): Uint8Array {
  return decodePrivateKey(input, stem).seed;
}
