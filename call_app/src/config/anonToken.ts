const ANON_TOKEN_KEY = 'call_app:anon_token';

export function getAnonToken(): string {
  let token = localStorage.getItem(ANON_TOKEN_KEY);
  if (!token) {
    token = crypto.randomUUID();
    localStorage.setItem(ANON_TOKEN_KEY, token);
  }
  return token;
}
