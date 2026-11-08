import type { Anekdot, Category, ExternalAnekdot, Feedback, Pagination } from "./types";

const apiBase = import.meta.env.VITE_API_BASE ?? "/api/v1";

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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  if (init?.body !== undefined) {
    headers.set("Content-Type", "application/json");
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

  submitFeedback: (message: string, contact: string) =>
    request<{ feedback: Feedback }>("/feedback", {
      method: "POST",
      body: JSON.stringify({ message, contact }),
    }),

  listFeedback: (token: string) =>
    request<{ feedback: Feedback[]; pagination: Pagination }>("/admin/feedback", {
      headers: { "X-Admin-Token": token },
    }),
};

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message;
  }

  return "Произошла непредвиденная ошибка.";
}
