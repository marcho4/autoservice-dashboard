"use client";

import { FormEvent, useState } from "react";
import ConfirmButton from "@/components/ConfirmButton";
import { Alert, Button, Field } from "@/components/ui";
import { FieldType, Service, ServiceInput } from "@/lib/api";

type DraftField = {
  key: string; // stable React key; the saved field ID once the field exists on the server
  id: string;
  label: string;
  question: string;
  type: FieldType;
  required: boolean;
  optionsText: string;
};

const TYPE_LABELS: Record<FieldType, string> = {
  text: "Текст",
  number: "Число",
  choice: "Выбор из вариантов",
};

const inputClass =
  "w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm outline-none focus:border-zinc-900 focus:ring-1 focus:ring-zinc-900";

let draftSeq = 0;

function toDraft(service: Service | null): DraftField[] {
  return (service?.fields ?? []).map((f) => ({ ...f, key: f.id, optionsText: f.options.join("\n") }));
}

// Editor of one service and its form: the questions the bot asks a client who needs this service.
export default function ServiceEditor({
  service,
  onSave,
  onDelete,
  onCancel,
}: {
  service: Service | null;
  onSave: (input: ServiceInput) => Promise<string | null>;
  onDelete?: () => Promise<string | null>;
  onCancel: () => void;
}) {
  const [fields, setFields] = useState<DraftField[]>(() => toDraft(service));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  function patch(key: string, change: Partial<DraftField>) {
    setFields((list) => list.map((f) => (f.key === key ? { ...f, ...change } : f)));
  }

  function move(index: number, delta: number) {
    setFields((list) => {
      const next = [...list];
      const [item] = next.splice(index, 1);
      next.splice(index + delta, 0, item);
      return next;
    });
  }

  function addField() {
    draftSeq += 1;
    setFields((list) => [
      ...list,
      { key: `new-${draftSeq}`, id: "", label: "", question: "", type: "text", required: true, optionsText: "" },
    ]);
  }

  async function finish(action: () => Promise<string | null>) {
    setBusy(true);
    setError("");
    const failure = await action();
    if (failure) {
      setError(failure);
      setBusy(false);
    }
  }

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    finish(() =>
      onSave({
        name: String(form.get("name")),
        description: String(form.get("description")),
        is_active: service?.is_active ?? true,
        fields: fields.map((f) => ({
          id: f.id,
          label: f.label,
          question: f.question,
          type: f.type,
          required: f.required,
          options:
            f.type === "choice"
              ? f.optionsText
                  .split("\n")
                  .map((o) => o.trim())
                  .filter(Boolean)
              : [],
        })),
      }),
    );
  }

  return (
    <form onSubmit={submit} className="space-y-5">
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Название услуги" name="name" required maxLength={60} defaultValue={service?.name} placeholder="Замена масла" />
        <Field
          label="Когда клиенту нужна эта услуга"
          name="description"
          maxLength={500}
          defaultValue={service?.description}
          placeholder="Плановое ТО, замена масла и фильтров"
          hint="По этому описанию бот понимает, что клиент пишет именно про эту услугу"
        />
      </div>

      <div>
        <div className="mb-2 text-sm font-medium text-zinc-700">Что бот спросит у клиента</div>
        <div className="space-y-3">
          {fields.length === 0 && (
            <p className="rounded-md border border-dashed border-zinc-300 px-4 py-3 text-sm text-zinc-500">
              Вопросов нет — бот запишет услугу и сразу спросит телефон.
            </p>
          )}
          {fields.map((f, i) => (
            <div key={f.key} className="rounded-md border border-zinc-200 bg-zinc-50 p-3">
              <div className="flex flex-wrap items-end gap-3">
                <label className="min-w-40 flex-1">
                  <span className="mb-1 block text-xs font-medium text-zinc-600">Название поля в заявке</span>
                  <input
                    className={inputClass}
                    value={f.label}
                    required
                    maxLength={60}
                    placeholder="Автомобиль"
                    onChange={(e) => patch(f.key, { label: e.target.value })}
                  />
                </label>
                <label>
                  <span className="mb-1 block text-xs font-medium text-zinc-600">Ответ</span>
                  <select className={inputClass} value={f.type} onChange={(e) => patch(f.key, { type: e.target.value as FieldType })}>
                    {Object.entries(TYPE_LABELS).map(([value, label]) => (
                      <option key={value} value={value}>
                        {label}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="flex items-center gap-2 pb-2 text-sm text-zinc-700">
                  <input type="checkbox" checked={f.required} onChange={(e) => patch(f.key, { required: e.target.checked })} />
                  Обязательно
                </label>
                <div className="flex gap-1 pb-0.5">
                  <IconButton label="Выше" disabled={i === 0} onClick={() => move(i, -1)}>
                    ↑
                  </IconButton>
                  <IconButton label="Ниже" disabled={i === fields.length - 1} onClick={() => move(i, 1)}>
                    ↓
                  </IconButton>
                  <IconButton label="Удалить поле" onClick={() => setFields((list) => list.filter((x) => x.key !== f.key))}>
                    ✕
                  </IconButton>
                </div>
              </div>
              <label className="mt-3 block">
                <span className="mb-1 block text-xs font-medium text-zinc-600">Вопрос бота</span>
                <input
                  className={inputClass}
                  value={f.question}
                  maxLength={300}
                  placeholder={f.label ? `Укажите: ${f.label.charAt(0).toLowerCase()}${f.label.slice(1)}` : "Какой у вас автомобиль?"}
                  onChange={(e) => patch(f.key, { question: e.target.value })}
                />
              </label>
              {f.type === "choice" && (
                <label className="mt-3 block">
                  <span className="mb-1 block text-xs font-medium text-zinc-600">Варианты ответа — по одному в строке, от 2 до 10</span>
                  <textarea
                    className={inputClass}
                    rows={Math.max(3, f.optionsText.split("\n").length)}
                    value={f.optionsText}
                    required
                    placeholder={"Сезонная смена шин\nРемонт прокола"}
                    onChange={(e) => patch(f.key, { optionsText: e.target.value })}
                  />
                </label>
              )}
            </div>
          ))}
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-3">
          <Button type="button" variant="secondary" onClick={addField} disabled={fields.length >= 15}>
            + Добавить вопрос
          </Button>
          <span className="text-xs text-zinc-500">Телефон бот спрашивает всегда — добавлять его не нужно.</span>
        </div>
      </div>

      {error && <Alert>{error}</Alert>}

      <div className="flex flex-wrap items-center gap-2 border-t border-zinc-100 pt-4">
        <Button type="submit" disabled={busy}>
          {service ? "Сохранить" : "Добавить услугу"}
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel}>
          Отмена
        </Button>
        {onDelete && (
          <span className="ml-auto">
            <ConfirmButton label="Удалить услугу" confirmLabel="Да, удалить" disabled={busy} onConfirm={() => finish(onDelete)} />
          </span>
        )}
      </div>
    </form>
  );
}

function IconButton({
  label,
  children,
  ...props
}: { label: string; children: React.ReactNode } & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      className="h-9 w-9 rounded-md border border-zinc-300 bg-white text-sm text-zinc-700 hover:bg-zinc-100 disabled:cursor-not-allowed disabled:text-zinc-300"
      {...props}
    >
      {children}
    </button>
  );
}
