export type RequestStatus = "new" | "in_progress" | "closed" | "rejected" | "cancelled";

export type Employee = {
  id: string;
  email: string;
  full_name: string;
  created_at: string;
  updated_at: string;
};

export type Autoservice = {
  id: string;
  name: string;
  address: string;
  phone: string;
  description: string;
  created_at: string;
  updated_at: string;
};

export type Bot = {
  username: string;
  telegram_bot_id: number;
  masked_token: string;
  connected_at: string;
  updated_at: string;
};

export type Client = {
  id: string;
  telegram_id: number;
  telegram_username: string;
  name: string;
  phone: string;
  created_at: string;
};

export type ClientSummary = Client & { requests_count: number; last_request_at: string | null };

export type ServiceRequest = {
  id: string;
  status: RequestStatus;
  car_brand: string;
  car_model: string;
  description: string;
  phone: string;
  created_at: string;
  updated_at: string;
};

export type RequestWithClient = ServiceRequest & { client: Client };

export type List<T> = { items: T[]; total: number };

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public field?: string,
  ) {
    super(message);
  }
}

const MESSAGES: Record<string, string> = {
  invalid_credentials: "Неверный email или пароль",
  email_taken: "Этот email уже зарегистрирован",
  autoservice_exists: "У вас уже есть автосервис",
  no_autoservice: "Сначала зарегистрируйте автосервис",
  bot_already_linked: "Этот бот уже подключён к другому автосервису",
  bot_not_connected: "Бот не подключён",
  invalid_bot_token: "Telegram отклонил токен — проверьте его в @BotFather",
  invalid_transition: "Такой переход статуса недопустим",
  request_not_editable: "Заявку уже нельзя изменить",
  not_found: "Не найдено",
  backend_unavailable: "Сервер недоступен, попробуйте позже",
  internal: "Внутренняя ошибка сервера",
};

const FIELDS: Record<string, string> = {
  email: "Email",
  password: "Пароль",
  full_name: "Имя",
  name: "Название",
  address: "Адрес",
  phone: "Телефон",
  description: "Описание",
  token: "Токен",
};

export function errorText(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.code === "validation_error" && e.field) {
      const reason = e.message.split(": ").slice(1).join(": ");
      return `${FIELDS[e.field] ?? e.field}: ${reason}`;
    }
    return MESSAGES[e.code] ?? e.message;
  }
  return "Что-то пошло не так";
}

async function request<T>(url: string, method: string, body?: unknown): Promise<T> {
  const res = await fetch(url, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });
  if (res.status === 401 && !url.startsWith("/api/session")) {
    // Session expired or revoked: full reload drops all client state.
    // eslint-disable-next-line @next/next/no-location-assign-relative-destination
    window.location.href = "/login";
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const err = data?.error ?? {};
    throw new ApiError(res.status, err.code ?? "unknown", err.message ?? res.statusText, err.field);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>(`/api/backend/${path}`, "GET"),
  post: <T>(path: string, body?: unknown) => request<T>(`/api/backend/${path}`, "POST", body),
  put: <T>(path: string, body?: unknown) => request<T>(`/api/backend/${path}`, "PUT", body),
  patch: <T>(path: string, body?: unknown) => request<T>(`/api/backend/${path}`, "PATCH", body),
  del: <T>(path: string, body?: unknown) => request<T>(`/api/backend/${path}`, "DELETE", body),

  login: (email: string, password: string) =>
    request<{ employee: Employee }>("/api/session", "POST", { email, password }),
  register: (email: string, password: string, full_name: string) =>
    request<{ employee: Employee }>("/api/session?register=1", "POST", { email, password, full_name }),
  logout: () => request<void>("/api/session", "DELETE"),
};

export const STATUS_LABELS: Record<RequestStatus, string> = {
  new: "Новая",
  in_progress: "В работе",
  closed: "Закрыта",
  rejected: "Отклонена",
  cancelled: "Отменена",
};

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
