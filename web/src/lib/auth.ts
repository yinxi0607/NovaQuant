import { API_BASE, APIError, AuthConfig, AuthLoginResponse, AuthSession } from "./api";
import { clearAuthState, readAuthToken, readAuthUser, writeAuthState } from "./authStore";

export const AUTH_ENABLED =
  import.meta.env.MODE !== "test" && (import.meta.env.VITE_AUTH_ENABLED ?? "false") === "true";

export function getStoredToken(): string | null {
  return readAuthToken();
}

export function getStoredUser(): string | null {
  return readAuthUser();
}

export function storeAuth(token: string, session: AuthSession) {
  writeAuthState(token, session.username);
}

export function clearAuth() {
  clearAuthState();
}

export async function fetchAuthConfig(): Promise<AuthConfig> {
  const response = await fetch(`${API_BASE}/auth/config`);
  if (!response.ok) {
    throw new APIError(response.status, await response.text());
  }
  return response.json() as Promise<AuthConfig>;
}

export async function login(username: string, password: string): Promise<AuthLoginResponse> {
  const config = await fetchAuthConfig();
  const body = config.enabled
    ? {
        username,
        encrypted_password: await encryptPassword(password, config.public_key),
      }
    : {
        username,
        password,
      };
  const response = await fetch(`${API_BASE}/auth/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw new APIError(response.status, await response.text());
  }
  return response.json() as Promise<AuthLoginResponse>;
}

async function encryptPassword(password: string, pem: string): Promise<string> {
  if (!window.crypto?.subtle) {
    throw new Error("Web Crypto is not available in this browser");
  }
  const key = await window.crypto.subtle.importKey(
    "spki",
    pemToArrayBuffer(pem),
    { name: "RSA-OAEP", hash: "SHA-256" },
    false,
    ["encrypt"],
  );
  const encrypted = await window.crypto.subtle.encrypt({ name: "RSA-OAEP" }, key, new TextEncoder().encode(password));
  return arrayBufferToBase64(encrypted);
}

function pemToArrayBuffer(pem: string): ArrayBuffer {
  const base64 = pem
    .replace("-----BEGIN PUBLIC KEY-----", "")
    .replace("-----END PUBLIC KEY-----", "")
    .replace(/\s+/g, "");
  const binary = window.atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes.buffer;
}

function arrayBufferToBase64(value: ArrayBuffer): string {
  const bytes = new Uint8Array(value);
  let binary = "";
  bytes.forEach((item) => {
    binary += String.fromCharCode(item);
  });
  return window.btoa(binary);
}
