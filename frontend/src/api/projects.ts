// Проектные API (BC-001): создание, список, детали, расчёт, экспорт,
// управление участниками (Phase C, EDR-0008).

import { del, get, patch, post } from './client'
import type {
  Calculation,
  CommentRequest,
  CreateProjectRequest,
  MemberRequest,
  Project,
  ProjectComment,
  ProjectMember,
} from './types'

export const projectsApi = {
  list: () => get<Project[]>('/api/v1/projects'),

  create: (body: CreateProjectRequest) => post<Project>('/api/v1/projects', body),

  get: (id: string) => get<Project>(`/api/v1/projects/${id}`),

  calculate: (id: string, body: unknown) =>
    post<Calculation>(`/api/v1/projects/${id}/calculate`, body),

  exportUrl: (id: string) => `/api/v1/projects/${id}/export`,

  // ---- Участники (EDR-0008) ----
  listMembers: (id: string) => get<ProjectMember[]>(`/api/v1/projects/${id}/members`),

  addMember: (id: string, body: MemberRequest) =>
    post<{ status: string }>(`/api/v1/projects/${id}/members`, body),

  updateMemberRole: (id: string, userID: string, role: ProjectMember['role']) =>
    patch<{ status: string }>(`/api/v1/projects/${id}/members/${userID}`, { role }),

  removeMember: (id: string, userID: string) =>
    del(`/api/v1/projects/${id}/members/${userID}`),

  // ---- Комментарии (EDR-0009) ----
  listComments: (id: string) => get<ProjectComment[]>(`/api/v1/projects/${id}/comments`),

  addComment: (id: string, body: CommentRequest) =>
    post<ProjectComment>(`/api/v1/projects/${id}/comments`, body),

  deleteComment: (id: string, commentID: string) =>
    del(`/api/v1/projects/${id}/comments/${commentID}`),
}