const AUTH_TOKEN_KEY = "novaquant.auth.token";
const AUTH_USER_KEY = "novaquant.auth.user";

let currentToken: string | null = null;
let currentUser: string | null = null;

function canUseStorage(): boolean {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

export function readAuthToken(): string | null {
  if (currentToken) {
    return currentToken;
  }
  if (!canUseStorage()) {
    return null;
  }
  currentToken = window.localStorage.getItem(AUTH_TOKEN_KEY);
  return currentToken;
}

export function readAuthUser(): string | null {
  if (currentUser) {
    return currentUser;
  }
  if (!canUseStorage()) {
    return null;
  }
  currentUser = window.localStorage.getItem(AUTH_USER_KEY);
  return currentUser;
}

export function writeAuthState(token: string, username: string) {
  currentToken = token;
  currentUser = username;
  if (!canUseStorage()) {
    return;
  }
  window.localStorage.setItem(AUTH_TOKEN_KEY, token);
  window.localStorage.setItem(AUTH_USER_KEY, username);
}

export function clearAuthState() {
  currentToken = null;
  currentUser = null;
  if (!canUseStorage()) {
    return;
  }
  window.localStorage.removeItem(AUTH_TOKEN_KEY);
  window.localStorage.removeItem(AUTH_USER_KEY);
}
