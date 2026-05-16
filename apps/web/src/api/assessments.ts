import { apiClient } from "./client";

export interface Framework {
  id: string;
  slug: string;
  name: string;
  description: string;
  version: string;
}

export interface AssessmentSummary {
  id: string;
  title: string;
  status: string;
  framework_id: string;
  created_at: string;
}

export interface AssessmentDetail {
  id: string;
  title: string;
  status: string;
  framework_id: string;
  progress: Array<{
    role: string;
    total: number;
    answered: number;
    pct: number;
  }>;
}

export interface RubricLevel {
  level: number;
  label: string;
  description: string;
}

export interface QuestionNode {
  id: string;
  slug: string;
  prompt: string;
  target_role: string;
  assigned_role: string;
  assigned_user_id?: string;
  rubric_levels: RubricLevel[];
}

export interface ResponseRecord {
  question_slug: string;
  level: number;
  free_text?: string;
  updated_at: string;
  assigned_role: string;
}

export interface UpsertResponsePayload {
  level: number;
  free_text?: string;
  evidence?: Array<{ kind: string; ref: string }>;
}

export const assessmentsApi = {
  listFrameworks: () => apiClient.get<Framework[]>("/v1/frameworks"),

  list: () => apiClient.get<AssessmentSummary[]>("/v1/assessments"),

  create: (frameworkId: string, title: string) =>
    apiClient.post<{ id: string; status: string }>("/v1/assessments", {
      framework_id: frameworkId,
      title,
    }),

  get: (id: string) => apiClient.get<AssessmentDetail>(`/v1/assessments/${id}`),

  start: (id: string) => apiClient.post(`/v1/assessments/${id}/start`),

  submit: (id: string) => apiClient.post(`/v1/assessments/${id}/submit`),

  assign: (id: string, assignments: Record<string, string>) =>
    apiClient.post(`/v1/assessments/${id}/assignments`, { assignments }),

  listQuestions: (assessmentId: string, role?: string) =>
    apiClient.get<QuestionNode[]>(
      `/v1/assessments/${assessmentId}/questions${role ? `?role=${role}` : ""}`,
    ),

  listResponses: (assessmentId: string, role?: string) =>
    apiClient.get<ResponseRecord[]>(
      `/v1/assessments/${assessmentId}/responses${role ? `?role=${role}` : ""}`,
    ),

  upsertResponse: (
    assessmentId: string,
    questionSlug: string,
    payload: UpsertResponsePayload,
  ) =>
    apiClient.put(
      `/v1/assessments/${assessmentId}/responses/${questionSlug}`,
      payload,
    ),

  deleteResponse: (assessmentId: string, questionSlug: string) =>
    apiClient.delete(
      `/v1/assessments/${assessmentId}/responses/${questionSlug}`,
    ),
};
