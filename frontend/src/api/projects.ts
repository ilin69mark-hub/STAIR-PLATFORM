// Проектные API (BC-001): создание, список, детали, расчёт, экспорт,
// управление участниками (Phase C, EDR-0008).

import { del, get, patch, post } from './client'
import type {
  ApprovalRequest,
  AssistantKind,
  AssistantResult,
  Calculation,
  CommentRequest,
  Configuration,
  ConfigurationApproval,
  CreateProjectRequest,
  MemberRequest,
  OptimizeResult,
  Project,
  ProjectComment,
  ProjectMember,
  ProjectReview,
  ReviewRequest,
} from '@shared/types'

export const projectsApi = {
  list: () => get<Project[]>('/api/v1/projects'),

  create: (body: CreateProjectRequest) => post<Project>('/api/v1/projects', body),

  get: (id: string) => get<Project>(`/api/v1/projects/${id}`),

  calculate: (id: string, body: unknown) =>
    post<Calculation>(`/api/v1/projects/${id}/calculate`, body),

  optimize: (id: string, body: unknown) =>
    post<OptimizeResult>(`/api/v1/projects/${id}/optimize`, body),

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

  // ---- Ревью (EDR-0010) ----
  requestReview: (id: string, body: ReviewRequest) =>
    post<ProjectReview>(`/api/v1/projects/${id}/review`, body),

  signOffReview: (id: string, reviewID: string, body: ReviewRequest) =>
    post<ProjectReview>(`/api/v1/projects/${id}/reviews/${reviewID}/sign-off`, body),

  requestChanges: (id: string, reviewID: string, body: ReviewRequest) =>
    post<ProjectReview>(`/api/v1/projects/${id}/reviews/${reviewID}/changes`, body),

  listReviews: (id: string) => get<ProjectReview[]>(`/api/v1/projects/${id}/reviews`),

  // ---- Утверждение конфигурации (EDR-0011) ----
  approveConfiguration: (id: string, configurationID: string, body: ApprovalRequest) =>
    post<ConfigurationApproval>(
      `/api/v1/projects/${id}/configurations/${configurationID}/approve`,
      body,
    ),

  getConfigurationApproval: (id: string, configurationID: string) =>
    get<ConfigurationApproval>(`/api/v1/projects/${id}/configurations/${configurationID}/approval`),

  listApprovals: (id: string) => get<ConfigurationApproval[]>(`/api/v1/projects/${id}/approvals`),

  // ---- Версионирование конфигурации (EDR-0012) ----
  listConfigurations: (id: string) => get<Configuration[]>(`/api/v1/projects/${id}/configurations`),

  getConfiguration: (id: string, configurationID: string) =>
    get<Configuration>(`/api/v1/projects/${id}/configurations/${configurationID}`),

  restoreConfiguration: (id: string, configurationID: string) =>
    post<Configuration>(`/api/v1/projects/${id}/configurations/${configurationID}/restore`, {}),

  // ---- AI-ассистенты (Phase D, EDR-0036): запрос к ассистенту kind ----
  assistant: (kind: AssistantKind, body: unknown) =>
    post<AssistantResult>(`/api/v1/assistant/${kind}`, body),
}