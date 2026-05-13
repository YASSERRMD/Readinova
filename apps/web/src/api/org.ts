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
  invite: (orgId: string, payload: InviteMemberPayload) =>
    apiClient.post<InviteMemberResponse>("/v1/invitations", payload),

  acceptInvitation: (token: string) =>
    apiClient.post(`/v1/invitations/${token}/accept`),

  patchOrg: (orgId: string, patch: Record<string, unknown>) =>
    apiClient.patch(`/v1/organisations/${orgId}`, patch),
};
