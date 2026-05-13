import { apiClient } from './client';

export interface Recommendation {
  id: string;
  dimension_slug: string;
  title: string;
  description: string;
  effort: 'low' | 'medium' | 'high';
  impact: 'low' | 'medium' | 'high';
  priority: number;
  wave: 1 | 2 | 3;
}

export interface RecommendationsResponse {
  total: number;
  recommendations: Recommendation[];
  waves: {
    wave_1: Recommendation[] | null;
    wave_2: Recommendation[] | null;
    wave_3: Recommendation[] | null;
  };
}

export const recommendApi = {
  get(assessmentId: string): Promise<RecommendationsResponse> {
    return apiClient.get(`/v1/assessments/${assessmentId}/recommendations`).then(r => r.data);
  },
};
