import { apiClient } from "./client";

export interface SignupPayload {
  org_name: string;
  org_slug: string;
  country_code: string;
  sector: string;
  size_band: string;
  email: string;
  password: string;
}

export interface AuthResponse {
  access_token: string;
  user_id: string;
  org_id: string;
  role: string;
}

export interface MeResponse {
  user_id: string;
  email: string;
  org_id: string;
  role: string;
}

export const authApi = {
  signup: (payload: SignupPayload) =>
    apiClient.post<AuthResponse>("/v1/organisations", payload),

  login: (email: string, password: string) =>
    apiClient.post<AuthResponse>("/v1/auth/login", { email, password }),

  refresh: () => apiClient.post<AuthResponse>("/v1/auth/refresh"),

  me: () => apiClient.get<MeResponse>("/v1/me"),
};
