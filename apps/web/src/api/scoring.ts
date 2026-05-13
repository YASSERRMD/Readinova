import { apiClient } from "./client";

export interface DerivedIndices {
  readiness_index: number;
  governance_risk_score: number;
  execution_capacity_score: number;
  value_realisation_score: number;
}

export interface ScoringResult {
  composite_layer_a: number;
  dimension_scores: Record<string, number>;
  sub_dimension_scores: Record<string, number>;
  binding_constraint_dimension: string;
  binding_constraint_score: number;
  derived: DerivedIndices;
  engine_version: string;
  framework_version: string;
}

export interface PerceptionGapResult {
  layer_a_score: number;
  layer_b_score: number;
  gap_score: number;
  master_composite: number;
  detail?: {
    dimension_scores_a?: Record<string, number>;
    dimension_scores_b?: Record<string, number>;
  };
}

export const scoringApi = {
  trigger: (assessmentId: string) =>
    apiClient.post<{ run_id: string; composite_layer_a: number }>(
      `/v1/assessments/${assessmentId}/score`,
    ),

  getResult: (assessmentId: string) =>
    apiClient.get<ScoringResult>(`/v1/assessments/${assessmentId}/score`),

  runPerceptionGap: (assessmentId: string) =>
    apiClient.post<PerceptionGapResult>(`/v1/assessments/${assessmentId}/perception-gap`),

  getPerceptionGap: (assessmentId: string) =>
    apiClient.get<PerceptionGapResult>(`/v1/assessments/${assessmentId}/perception-gap`),
};
