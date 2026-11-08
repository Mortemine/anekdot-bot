export interface Category {
  id: number;
  name: string;
  anekdot_count: number;
}

export interface Anekdot {
  id: number;
  category_id: number;
  category_name: string;
  text: string;
}

export interface ExternalAnekdot {
  text: string;
  source: string;
}

export type FeedbackStatus = "new" | "read" | "archived";

export interface Feedback {
  id: number;
  message: string;
  contact: string | null;
  status: FeedbackStatus;
  created_at: string;
}

export interface Pagination {
  limit: number;
  offset: number;
}
