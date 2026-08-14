// Проектные API (BC-001): создание, список, детали, расчёт, экспорт.

import { get, post } from './client'
import type { Calculation, CreateProjectRequest, Project } from './types'

export const projectsApi = {
  list: () => get<Project[]>('/api/v1/projects'),

  create: (body: CreateProjectRequest) => post<Project>('/api/v1/projects', body),

  get: (id: string) => get<Project>(`/api/v1/projects/${id}`),

  calculate: (id: string, body: unknown) =>
    post<Calculation>(`/api/v1/projects/${id}/calculate`, body),

  exportUrl: (id: string) => `/api/v1/projects/${id}/export`,
}