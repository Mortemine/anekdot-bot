import type { Anekdot, AnekdotPage, AnekdotSort, AuthResult, Category, ExternalAnekdot, User } from "./types";

const apiBase = import.meta.env.VITE_API_BASE ?? "/api/v1";
const tokenKey = "anekdot.token";

export interface AnekdotListOptions {
  limit: number;
  offset: number;
  sort: AnekdotSort;
}

interface ErrorBody {
  error?: {
    code?: string;
    message?: string;
  };
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(message: string, status: number, code: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export function getToken(): string | null {
  try {
    return window.localStorage.getItem(tokenKey);
  } catch {
    return null;
  }
}

export function setToken(token: string | null): void {
  try {
    if (token === null) {
      window.localStorage.removeItem(tokenKey);
      return;
    }

    window.localStorage.setItem(tokenKey, token);
  } catch {
    return;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  if (init?.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  const token = getToken();
  if (token !== null) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  let response: Response;
  try {
    response = await fetch(`${apiBase}${path}`, { ...init, headers });
  } catch {
    throw new ApiError("Сервер недоступен. Проверьте соединение.", 0, "network_error");
  }

  const payload = (await response.json().catch(() => null)) as (T & ErrorBody) | null;

  if (!response.ok) {
    throw new ApiError(
      payload?.error?.message ?? "Не удалось выполнить запрос.",
      response.status,
      payload?.error?.code ?? "unknown_error",
    );
  }

  return payload as T;
}

export const api = {
  listCategories: () => request<{ categories: Category[] }>("/categories"),

  randomAnekdot: (categoryId: number | null) =>
    request<{ anekdot: Anekdot }>(
      categoryId === null ? "/anekdots/random" : `/anekdots/random?category_id=${categoryId}`,
    ),

  externalAnekdot: () => request<{ anekdot: ExternalAnekdot }>("/anekdots/external"),

  register: (email: string, password: string, displayName: string) =>
    request<AuthResult>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password, display_name: displayName }),
    }),

  login: (email: string, password: string) =>
    request<AuthResult>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  logout: () => request<void>("/auth/logout", { method: "POST" }),

  me: () => request<{ user: User }>("/auth/me"),

  createAnekdot: (categoryId: number, text: string) =>
    request<{ anekdot: Anekdot }>("/anekdots", {
      method: "POST",
      body: JSON.stringify({ category_id: categoryId, text }),
    }),

  myAnekdots: () => request<{ anekdots: Anekdot[] }>("/anekdots/mine"),

  listAnekdots: (categoryId: number | null, options: AnekdotListOptions) => {
    const params = new URLSearchParams({
      limit: String(options.limit),
      offset: String(options.offset),
      sort: options.sort,
    });
    if (categoryId !== null) {
      params.set("category_id", String(categoryId));
    }

    return request<AnekdotPage>(`/anekdots?${params.toString()}`);
  },

  listLikes: () => request<{ anekdots: Anekdot[] }>("/likes"),

  addLike: (anekdotId: number) => request<void>(`/likes/${anekdotId}`, { method: "PUT" }),

  removeLike: (anekdotId: number) => request<void>(`/likes/${anekdotId}`, { method: "DELETE" }),

  listFavorites: () => request<{ anekdots: Anekdot[] }>("/favorites"),

  addFavorite: (anekdotId: number) => request<void>(`/favorites/${anekdotId}`, { method: "PUT" }),

  removeFavorite: (anekdotId: number) => request<void>(`/favorites/${anekdotId}`, { method: "DELETE" }),
};

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message;
  }

  return "Произошла непредвиденная ошибка.";
}
