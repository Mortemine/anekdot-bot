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
  like_count: number;
  favorite_count: number;
  author_id?: number;
  created_at: string;
}

export type AnekdotSort = "created" | "likes" | "favorites";

export interface AnekdotPage {
  anekdots: Anekdot[];
  total: number;
  limit: number;
  offset: number;
}

export interface ExternalAnekdot {
  text: string;
  source: string;
}

export interface User {
  id: number;
  email: string;
  display_name: string;
  created_at: string;
}

export interface AuthResult {
  user: User;
  token: string;
  expires_at: string;
}

export interface Collection {
  id: number;
  name: string;
  owner_id: number;
  owner_name: string;
  is_public: boolean;
  anekdot_count: number;
  contains_anekdot: boolean;
  favorited: boolean;
  created_at: string;
}

export interface CollectionDetail {
  collection: Collection;
  anekdots: Anekdot[];
}
