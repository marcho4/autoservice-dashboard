"use client";

import { FormEvent, useState } from "react";
import ConfirmButton from "@/components/ConfirmButton";
import { Alert, Button, Field, Icon, IconButton, Select, TextArea } from "@/components/ui";
import { FieldType, Service, ServiceInput } from "@/lib/api";

type DraftField = {
  key: string;
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

const MIN_OPTIONS = 2;
const MAX_OPTIONS = 10;

function parseOptions(text: string): string[] {
  return text
    .split("\n")
    .map((o) => o.trim())
    .filter(Boolean);
}

let draftSeq = 0;

function toDraft(service: Service | null): DraftField[] {
  return (service?.fields ?? []).map((f) => ({ ...f, key: f.id, optionsText: f.options.join("\n") }));
}

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
          options: f.type === "choice" ? parseOptions(f.optionsText) : [],
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
        <h3 className="mb-2 font-semibold">Что бот спросит у клиента</h3>
        <div className="space-y-2">
          {fields.length === 0 && (
            <p className="rounded-control border border-dashed border-line-strong px-4 py-3 text-ink-2">
              Вопросов нет — бот запишет услугу и сразу спросит телефон.
            </p>
          )}
          {fields.map((f, i) => {
            const optionCount = parseOptions(f.optionsText).length;
            const optionsInvalid = optionCount < MIN_OPTIONS || optionCount > MAX_OPTIONS;
            return (
              <div key={f.key} role="group" aria-label={`Вопрос ${i + 1}`} className="space-y-3 rounded-control border border-line bg-paper p-3 sm:p-4">
                <div className="flex items-center gap-2">
                  <span className="mr-auto text-meta font-semibold text-ink-2">Вопрос {i + 1}</span>
                  <IconButton label="Выше" disabled={i === 0} onClick={() => move(i, -1)}>
                    <Icon name="up" />
                  </IconButton>
                  <IconButton label="Ниже" disabled={i === fields.length - 1} onClick={() => move(i, 1)}>
                    <Icon name="down" />
                  </IconButton>
                  <IconButton label="Удалить вопрос" className="ml-3" onClick={() => setFields((list) => list.filter((x) => x.key !== f.key))}>
                    <Icon name="close" />
                  </IconButton>
                </div>
                <div className="grid items-end gap-3 sm:grid-cols-[1fr_auto_auto]">
                  <Field
                    label="Название поля в заявке"
                    value={f.label}
                    required
                    maxLength={60}
                    placeholder="Автомобиль"
                    onChange={(e) => patch(f.key, { label: e.target.value })}
                  />
                  <Select label="Ответ" value={f.type} onChange={(e) => patch(f.key, { type: e.target.value as FieldType })}>
                    {Object.entries(TYPE_LABELS).map(([value, label]) => (
                      <option key={value} value={value}>
                        {label}
                      </option>
                    ))}
                  </Select>
                  <label className="flex min-h-control items-center gap-2.5">
                    <input
                      type="checkbox"
                      className="size-5 accent-ink"
                      checked={f.required}
                      onChange={(e) => patch(f.key, { required: e.target.checked })}
                    />
                    Обязательно
                  </label>
                </div>
                <Field
                  label="Вопрос бота"
                  value={f.question}
                  maxLength={300}
                  placeholder={f.label ? `Укажите: ${f.label.charAt(0).toLowerCase()}${f.label.slice(1)}` : "Какой у вас автомобиль?"}
                  onChange={(e) => patch(f.key, { question: e.target.value })}
                />
                {f.type === "choice" && (
                  <TextArea
                    label="Варианты ответа — по одному в строке"
                    rows={Math.max(3, f.optionsText.split("\n").length)}
                    value={f.optionsText}
                    required
                    placeholder={"Сезонная смена шин\nРемонт прокола"}
                    invalid={optionsInvalid}
                    hint={`Вариантов: ${optionCount} — нужно от ${MIN_OPTIONS} до ${MAX_OPTIONS}`}
                    onChange={(e) => patch(f.key, { optionsText: e.target.value })}
                  />
                )}
              </div>
            );
          })}
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
          <Button type="button" variant="secondary" onClick={addField} disabled={fields.length >= 15}>
            Добавить вопрос
          </Button>
          <span className="text-meta text-ink-3">Телефон бот спрашивает всегда — добавлять его не нужно.</span>
        </div>
      </div>

      {error && <Alert>{error}</Alert>}

      <div className="flex flex-wrap items-center gap-2 border-t border-line pt-4">
        <Button type="submit" disabled={busy}>
          {service ? "Сохранить" : "Добавить услугу"}
        </Button>
        <Button type="button" variant="ghost" onClick={onCancel}>
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
