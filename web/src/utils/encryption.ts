// Encryption utilities for secure credential handling
// Uses AES-256-GCM for symmetric encryption
// Password/key encryption happens before sending to backend; backend stores encrypted values

// Generate a cryptographically secure random encryption key
export async function generateEncryptionKey(): Promise<CryptoKey> {
  return crypto.subtle.generateKey(
    {
      name: 'AES-GCM',
      length: 256,
    },
    true, // extractable
    ['encrypt', 'decrypt']
  );
}

// Export key to base64 for storage/transmission
export async function exportKey(key: CryptoKey): Promise<string> {
  const exported = await crypto.subtle.exportKey('raw', key);
  return btoa(String.fromCharCode(...new Uint8Array(exported)));
}

// Import key from base64
export async function importKey(base64Key: string): Promise<CryptoKey> {
  const keyData = Uint8Array.from(atob(base64Key), c => c.charCodeAt(0));
  return crypto.subtle.importKey(
    'raw',
    keyData,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  );
}

// Encrypt a string value
export async function encrypt(plaintext: string, key: CryptoKey): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(plaintext);
  
  // Generate random IV (12 bytes for AES-GCM)
  const iv = crypto.getRandomValues(new Uint8Array(12));
  
  const encrypted = await crypto.subtle.encrypt(
    {
      name: 'AES-GCM',
      iv: iv,
    },
    key,
    data
  );
  
  // Combine IV + ciphertext and encode as base64
  const combined = new Uint8Array(iv.length + encrypted.byteLength);
  combined.set(iv);
  combined.set(new Uint8Array(encrypted), iv.length);
  
  return btoa(String.fromCharCode(...combined));
}

// Decrypt a string value
export async function decrypt(ciphertext: string, key: CryptoKey): Promise<string> {
  const combined = Uint8Array.from(atob(ciphertext), c => c.charCodeAt(0));
  
  // Extract IV (first 12 bytes) and ciphertext
  const iv = combined.slice(0, 12);
  const data = combined.slice(12);
  
  const decrypted = await crypto.subtle.decrypt(
    {
      name: 'AES-GCM',
      iv: iv,
    },
    key,
    data
  );
  
  const decoder = new TextDecoder();
  return decoder.decode(decrypted);
}

// ============================================
// High-level API for credential encryption
// ============================================

// Session encryption key (generated once per session)
let sessionKey: CryptoKey | null = null;
let sessionKeyBase64: string | null = null;

// Initialize or get session encryption key
export async function getSessionKey(): Promise<{ key: CryptoKey; base64: string }> {
  if (!sessionKey || !sessionKeyBase64) {
    sessionKey = await generateEncryptionKey();
    sessionKeyBase64 = await exportKey(sessionKey);
  }
  return { key: sessionKey, base64: sessionKeyBase64 };
}

// Encrypt credential for transmission to backend
export async function encryptCredential(value: string): Promise<{ encrypted: string; key: string }> {
  if (!value) {
    return { encrypted: '', key: '' };
  }
  const { key, base64 } = await getSessionKey();
  const encrypted = await encrypt(value, key);
  return { encrypted, key: base64 };
}

// Encrypt multiple credentials
export async function encryptCredentials(
  credentials: Record<string, string | undefined>
): Promise<{ encrypted: Record<string, string>; key: string }> {
  const { key, base64 } = await getSessionKey();
  const encrypted: Record<string, string> = {};
  
  for (const [field, value] of Object.entries(credentials)) {
    if (value) {
      encrypted[field] = await encrypt(value, key);
    }
  }
  
  return { encrypted, key: base64 };
}

// ============================================
// Credential field markers
// ============================================

// Fields that should be encrypted before transmission
export const SENSITIVE_FIELDS = [
  'password',
  'ssh_password',
  'ssh_private_key',
  'ssh_key_passphrase',
  'redis_password',
  'redis_tls_key',
  'tls_key',
  'private_key',
  'passphrase',
  'secret',
  'api_key',
  'token',
];

// Check if a field name indicates sensitive data
export function isSensitiveField(fieldName: string): boolean {
  const lowerName = fieldName.toLowerCase();
  return SENSITIVE_FIELDS.some(sensitive => lowerName.includes(sensitive));
}

// Encrypt all sensitive fields in an object
export async function encryptSensitiveFields<T extends Record<string, any>>(
  data: T
): Promise<{ data: T; encryptionKey: string }> {
  const { key, base64 } = await getSessionKey();
  const encrypted = { ...data };
  
  for (const [field, value] of Object.entries(data)) {
    if (typeof value === 'string' && value && isSensitiveField(field)) {
      (encrypted as any)[field] = await encrypt(value, key);
    } else if (typeof value === 'object' && value !== null) {
      // Recursively encrypt nested objects
      const result = await encryptSensitiveFields(value);
      (encrypted as any)[field] = result.data;
    }
  }
  
  return { data: encrypted, encryptionKey: base64 };
}

// ============================================
// Type definitions
// ============================================

export interface EncryptedCredentials {
  // The encrypted value (base64-encoded IV + ciphertext)
  value: string;
  // Indicator that this is encrypted
  encrypted: true;
}

export interface CredentialPayload {
  // Encrypted credentials
  credentials: Record<string, string>;
  // Encryption key for the backend to decrypt (base64)
  encryption_key: string;
}
