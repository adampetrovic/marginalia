// TanStack Query hooks for every Marginalia API endpoint.

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryOptions,
} from '@tanstack/react-query';
import { api } from './client';
import type {
  ApiToken,
  ApiTokenCreated,
  Document,
  ExportResult,
  Highlight,
  ReadeckIntegration,
  ReviewRating,
  ReviewResponse,
  Source,
  Stats,
  SyncLog,
  SyncResult,
  Template,
  TemplatePreview,
  TemplateType,
} from './types';

export const queryKeys = {
  stats: ['stats'] as const,
  sources: ['sources'] as const,
  syncStatus: ['sync', 'status'] as const,
  readeck: ['integrations', 'readeck'] as const,
  documents: (params?: { type?: string; q?: string }) =>
    ['documents', params ?? {}] as const,
  document: (id: string) => ['documents', id] as const,
  highlights: (params?: { document_id?: string; q?: string }) =>
    ['highlights', params ?? {}] as const,
  review: ['review'] as const,
  templates: ['templates'] as const,
  template: (id: string) => ['templates', id] as const,
  tokens: ['tokens'] as const,
};

function buildQuery(params?: Record<string, string | undefined>): string {
  if (!params) return '';
  const search = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v) search.set(k, v);
  });
  const str = search.toString();
  return str ? `?${str}` : '';
}

// --- Stats ---
export function useStats() {
  return useQuery({
    queryKey: queryKeys.stats,
    queryFn: () => api.get<Stats>('/stats'),
  });
}

// --- Sources ---
export function useSources() {
  return useQuery({
    queryKey: queryKeys.sources,
    queryFn: () => api.get<Source[]>('/sources'),
  });
}

// --- Sync ---
export function useSyncStatus() {
  return useQuery({
    queryKey: queryKeys.syncStatus,
    queryFn: () => api.get<SyncLog[]>('/sync/status'),
  });
}

export function useSync() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<SyncResult>('/sync'),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['documents'] });
      qc.invalidateQueries({ queryKey: queryKeys.stats });
      qc.invalidateQueries({ queryKey: queryKeys.syncStatus });
      qc.invalidateQueries({ queryKey: queryKeys.sources });
    },
  });
}

// --- Integrations ---
export function useReadeckIntegration() {
  return useQuery({
    queryKey: queryKeys.readeck,
    queryFn: () => api.get<ReadeckIntegration>('/integrations/readeck'),
  });
}

export function useUpdateReadeckIntegration() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { url: string; token: string }) =>
      api.put<{ configured: boolean }>('/integrations/readeck', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.readeck });
    },
  });
}

// --- Documents ---
export function useDocuments(
  params?: { type?: string; q?: string },
  options?: Partial<UseQueryOptions<Document[]>>,
) {
  return useQuery({
    queryKey: queryKeys.documents(params),
    queryFn: () =>
      api.get<Document[]>(`/documents${buildQuery(params)}`),
    ...options,
  });
}

export function useDocument(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.document(id ?? ''),
    queryFn: () => api.get<Document>(`/documents/${id}`),
    enabled: !!id,
  });
}

export function useUpdateDocument(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Partial<Document>) =>
      api.put<Document>(`/documents/${id}`, body),
    onSuccess: (doc) => {
      qc.setQueryData(queryKeys.document(id), doc);
      qc.invalidateQueries({ queryKey: ['documents'] });
      qc.invalidateQueries({ queryKey: queryKeys.stats });
    },
  });
}

export function useDeleteDocument() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<void>(`/documents/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['documents'] });
      qc.invalidateQueries({ queryKey: queryKeys.stats });
    },
  });
}

export function useExportDocument() {
  return useMutation({
    mutationFn: (id: string) =>
      api.get<ExportResult>(`/export/documents/${id}`),
  });
}

// --- Highlights ---
export function useHighlights(
  params?: { document_id?: string; q?: string },
  options?: Partial<UseQueryOptions<Highlight[]>>,
) {
  return useQuery({
    queryKey: queryKeys.highlights(params),
    queryFn: () => api.get<Highlight[]>(`/highlights${buildQuery(params)}`),
    ...options,
  });
}

export function useUpdateHighlight(documentId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      ...body
    }: {
      id: string;
      text?: string;
      note?: string;
      tags?: string[];
    }) => api.put<Highlight>(`/highlights/${id}`, body),
    onSuccess: () => {
      if (documentId) {
        qc.invalidateQueries({ queryKey: queryKeys.document(documentId) });
      }
      qc.invalidateQueries({ queryKey: ['highlights'] });
    },
  });
}

export function useDeleteHighlight(documentId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<void>(`/highlights/${id}`),
    onSuccess: () => {
      if (documentId) {
        qc.invalidateQueries({ queryKey: queryKeys.document(documentId) });
      }
      qc.invalidateQueries({ queryKey: ['highlights'] });
      qc.invalidateQueries({ queryKey: queryKeys.stats });
    },
  });
}

export function useFavoriteHighlight(documentId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      api.post<{ favorite: boolean }>(`/highlights/${id}/favorite`),
    onSuccess: () => {
      if (documentId) {
        qc.invalidateQueries({ queryKey: queryKeys.document(documentId) });
      }
      qc.invalidateQueries({ queryKey: ['highlights'] });
    },
  });
}

// --- Review ---
export function useReview() {
  return useQuery({
    queryKey: queryKeys.review,
    queryFn: () => api.get<ReviewResponse>('/review'),
  });
}

export function useRateReview() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, rating }: { id: string; rating: ReviewRating }) =>
      api.post<ReviewResponse>(`/review/${id}`, { rating }),
    onSuccess: (next) => {
      qc.setQueryData(queryKeys.review, next);
      qc.invalidateQueries({ queryKey: queryKeys.stats });
    },
  });
}

// --- Templates ---
export function useTemplates() {
  return useQuery({
    queryKey: queryKeys.templates,
    queryFn: () => api.get<Template[]>('/templates'),
  });
}

export function useTemplate(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.template(id ?? ''),
    queryFn: () => api.get<Template>(`/templates/${id}`),
    enabled: !!id,
  });
}

export function useCreateTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; type: TemplateType; page_template: string }) =>
      api.post<Template>('/templates', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.templates });
    },
  });
}

export function useUpdateTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...body }: { id: string } & Partial<Template>) =>
      api.put<Template>(`/templates/${id}`, body),
    onSuccess: (tpl) => {
      qc.invalidateQueries({ queryKey: queryKeys.templates });
      qc.setQueryData(queryKeys.template(tpl.id), tpl);
    },
  });
}

export function useDeleteTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<void>(`/templates/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.templates });
    },
  });
}

export function usePreviewTemplate() {
  return useMutation({
    mutationFn: (body: { page_template: string; type: TemplateType }) =>
      api.post<TemplatePreview>('/templates/preview', body),
  });
}

// --- Tokens ---
export function useTokens() {
  return useQuery({
    queryKey: queryKeys.tokens,
    queryFn: () => api.get<ApiToken[]>('/tokens'),
  });
}

export function useCreateToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string }) =>
      api.post<ApiTokenCreated>('/tokens', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.tokens });
    },
  });
}

export function useRevokeToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<void>(`/tokens/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.tokens });
    },
  });
}
