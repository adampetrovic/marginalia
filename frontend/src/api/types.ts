// TypeScript interfaces matching the Marginalia backend models.

export interface User {
  id: string;
  email: string;
  name: string;
  is_admin: boolean;
  created_at: string;
}

export interface Source {
  id: string;
  type: string;
  name: string;
  last_synced_at: string | null;
  created_at: string;
  updated_at: string;
}

export type DocumentType = 'book' | 'article' | 'podcast' | 'tweet';

export interface Document {
  id: string;
  source_id: string;
  type: DocumentType;
  title: string;
  author: string;
  url: string;
  image_url: string;
  source_url: string;
  category: string;
  tags: string[];
  metadata: Record<string, unknown>;
  favorite: boolean;
  last_highlighted_at: string | null;
  last_synced_at: string | null;
  created_at: string;
  updated_at: string;
  highlights?: Highlight[];
}

export interface Highlight {
  id: string;
  document_id: string;
  text: string;
  note: string;
  color: string;
  tags: string[];
  favorite: boolean;
  user_edited: boolean;
  location: string;
  location_type: string;
  chapter: string;
  page_number: number | null;
  percentage: number | null;
  highlighted_at: string | null;
  created_at: string;
  updated_at: string;
  review_state?: ReviewState;
}

export interface ReviewState {
  highlight_id: string;
  ease_factor: number;
  interval_days: number;
  repetitions: number;
  lapses: number;
  due_at: string | null;
  last_reviewed_at: string | null;
  last_rating: string;
  suspended: boolean;
  favorite: boolean;
}

export type TemplateType = 'book' | 'article';

export interface Template {
  id: string;
  name: string;
  type: TemplateType;
  page_template: string;
  highlight_template: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface SyncLog {
  id: number;
  source_id: string;
  status: 'started' | 'completed' | 'failed';
  documents_synced: number;
  highlights_synced: number;
  error: string;
  started_at: string;
  completed_at: string | null;
}

export interface ApiToken {
  id: string;
  name: string;
  prefix: string;
  last_used_at: string | null;
  created_at: string;
}

export interface ApiTokenCreated extends ApiToken {
  token: string;
}

export interface Stats {
  books: number;
  articles: number;
  highlights: number;
  due_reviews: number;
}

export interface SyncResult {
  readeck?: {
    status: string;
    documents: number;
    highlights: number;
  };
}

export interface ReadeckIntegration {
  url: string;
  token: string;
  configured: boolean;
}

export type ReviewRating = 'again' | 'hard' | 'good' | 'easy';

export interface ReviewStats {
  due: number;
  reviewed_today: number;
}

export interface ReviewCard {
  highlight: Highlight;
  document: Document;
  state: ReviewState;
  stats: ReviewStats;
  done?: false;
}

export interface ReviewDone {
  done: true;
}

export type ReviewResponse = ReviewCard | ReviewDone;

export interface TemplatePreview {
  content: string;
  title?: string;
}

export interface ExportResult {
  content: string;
  title: string;
  [key: string]: unknown;
}

export interface AuthResponse {
  user: User;
}
