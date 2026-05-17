import { apiClient } from "./client";

export interface InviteMemberPayload {
  email: string;
  role: string;
}

export interface InviteMemberResponse {
  token: string;
  invite_url: string;
}

export const orgApi = {
  invite: (payload: InviteMemberPayload) =>
    apiClient.post<InviteMemberResponse>("/v1/invitations", payload),

  acceptInvitation: (token: string, email: string, password: string) =>
    apiClient.post(`/v1/invitations/${token}/accept`, { email, password }),

  patchOrg: (orgId: string, patch: Record<string, unknown>) =>
    apiClient.patch(`/v1/organisations/${orgId}`, patch),
};
