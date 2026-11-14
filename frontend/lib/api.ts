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

export type BotStatus = "pending" | "running" | "error";

export type NotifyRecipient = { chat_id: number; name: string; created_at: string };

export type Bot = {
  username: string;
  telegram_bot_id: number;
  masked_token: string;
  link: string;
  status: BotStatus;
  status_error: string;
  status_at: string;
  notify_link: string;
  recipients: NotifyRecipient[];
  connected_at: string;
  updated_at: string;
};

export type FieldType = "text" | "number" | "choice";

export type FormField = {
  id: string;
  label: string;
  question: string;
  type: FieldType;
  required: boolean;
  options: string[];
};

export type Service = {
  id: string;
  name: string;
  description: string;
  is_active: boolean;
  fields: FormField[];
  created_at: string;
  updated_at: string;
};

export type ServiceInput = Pick<Service, "name" | "description" | "is_active" | "fields">;

export type Client = {
  id: string;
  telegram_id: number;
  telegram_username: string;
  name: string;
  phone: string;
  created_at: string;
};

export type ClientSummary = Client & { requests_count: number; last_request_at: string | null };

export type Answer = { label: string; value: string };

export type ServiceRequest = {
  id: string;
  number: number;
  status: RequestStatus;
  service_id: string | null;
  service_name: string;
  answers: Answer[];
  message: string;
  phone: string;
  note: string;
  created_at: string;
  updated_at: string;
};

export type RequestWithClient = ServiceRequest & { client: Client };

export type RequestPage = { items: RequestWithClient[]; total: number; counts: Record<RequestStatus, number> };

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
  service_limit: "Достигнут лимит: не больше 30 услуг",
  services_exist: "Стандартные услуги можно добавить только в пустой список",
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
  autoservice_name: "Название автосервиса",
  note: "Заметка",
};

const REASONS: [RegExp, string][] = [
  [/^must be 1-(\d+) characters$/, "от 1 до $1 символов"],
  [/^must be at most (\d+) characters$/, "не длиннее $1 символов"],
  [/^must be at least (\d+) characters$/, "не короче $1 символов"],
  [/^must be a valid email address$/, "укажите настоящий адрес"],
  [/^must be a valid phone number$/, "укажите номер в формате +7 900 000-00-00"],
  [/^has invalid format.*$/, "неверный формат, ожидается 123456789:AAH…"],
  [/^label must be 1-(\d+) characters$/, "название поля — от 1 до $1 символов"],
  [/^question must be at most (\d+) characters$/, "вопрос не длиннее $1 символов"],
  [/^option must be at most (\d+) characters$/, "вариант не длиннее $1 символов"],
  [/^choice must have 2-(\d+) options$/, "нужно от 2 до $1 вариантов ответа"],
  [/^must have at most (\d+) items$/, "не больше $1"],
];

function reasonText(reason: string): string {
  for (const [re, text] of REASONS) {
    if (re.test(reason)) return reason.replace(re, text);
  }
  return reason;
}

function fieldName(field: string): string {
  const m = /^fields\[(\d+)\]$/.exec(field);
  if (m) return `Поле №${Number(m[1]) + 1}`;
  if (field === "fields") return "Поля формы";
  return FIELDS[field] ?? field;
}

export function errorText(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.code === "validation_error" && e.field) {
      const reason = e.message.split(": ").slice(1).join(": ");
      return `${fieldName(e.field)}: ${reasonText(reason)}`;
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
  register: (email: string, password: string, full_name: string, autoservice_name: string) =>
    request<{ employee: Employee }>("/api/session?register=1", "POST", { email, password, full_name, autoservice_name }),
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

const TIME: Intl.DateTimeFormatOptions = { hour: "2-digit", minute: "2-digit" };

export function formatWhen(iso: string, now = new Date()): string {
  const date = new Date(iso);
  const minutes = Math.floor((now.getTime() - date.getTime()) / 60000);
  if (minutes >= 0 && minutes < 1) return "только что";
  if (minutes >= 1 && minutes < 60) return `${minutes} мин назад`;

  const time = date.toLocaleTimeString("ru-RU", TIME);
  const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const days = Math.round((startOfDay(now) - startOfDay(date)) / 86400000);
  if (days === 0) return `сегодня, ${time}`;
  if (days === 1) return `вчера, ${time}`;
  if (date.getFullYear() === now.getFullYear()) {
    return `${date.toLocaleDateString("ru-RU", { day: "numeric", month: "long" })}, ${time}`;
  }
  return date.toLocaleDateString("ru-RU", { day: "numeric", month: "long", year: "numeric" });
}

const pluralRules = new Intl.PluralRules("ru-RU");

export function requestsCount(n: number): string {
  const forms: Record<string, string> = { one: "заявка", few: "заявки" };
  return `${n} ${forms[pluralRules.select(n)] ?? "заявок"}`;
}

export function formatPhone(phone: string): string {
  const m = /^\+7(\d{3})(\d{3})(\d{2})(\d{2})$/.exec(phone);
  return m ? `+7 ${m[1]} ${m[2]}-${m[3]}-${m[4]}` : phone;
}

export function telHref(phone: string): string {
  return `tel:${phone.replace(/[^+\d]/g, "")}`;
}

export function clientName(c: Client): string {
  return c.name || (c.telegram_username ? `@${c.telegram_username}` : `ID ${c.telegram_id}`);
}

export function answersDigest(r: ServiceRequest): string {
  return r.answers.map((a) => a.value).join(" · ") || r.message;
}

export const BOT_ERRORS: Record<string, string> = {
  unauthorized: "Telegram отклонил токен — возможно, его перевыпустили в @BotFather. Вставьте новый токен.",
  conflict: "Сообщения этого бота уже получает другая программа. Отключите её или создайте для заявок отдельного бота.",
  unavailable: "Нет связи с Telegram. Бот заработает сам, как только связь восстановится.",
};
