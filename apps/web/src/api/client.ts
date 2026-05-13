import axios from "axios";

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? "http://localhost:8080",
  withCredentials: true,
  headers: { "Content-Type": "application/json" },
});

// Attach access token from in-memory store on each request.
apiClient.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

let _accessToken: string | null = null;

export function setAccessToken(t: string | null) {
  _accessToken = t;
}

export function getAccessToken(): string | null {
  return _accessToken;
}
