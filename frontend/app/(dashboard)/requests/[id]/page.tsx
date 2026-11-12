"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { Alert, Button, Card, Loading, PageTitle, StatusBadge } from "@/components/ui";
import { api, clientName, errorText, formatDate, formatPhone, RequestWithClient, telHref } from "@/lib/api";

export default function RequestPage() {
  const { id } = useParams<{ id: string }>();
  const [req, setReq] = useState<RequestWithClient | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<string | null>(null);

  useEffect(() => {
    api.get<RequestWithClient>(`requests/${id}`).then(setReq).catch((e) => setError(errorText(e)));
  }, [id]);

  async function run(call: () => Promise<RequestWithClient>) {
    setBusy(true);
    setError("");
    try {
      setReq(await call());
      return true;
    } catch (e) {
      setError(errorText(e));
      return false;
    } finally {
      setBusy(false);
    }
  }

  const act = (action: "take" | "reject" | "close") => run(() => api.post<RequestWithClient>(`requests/${id}/${action}`));

  async function saveNote() {
    if (await run(() => api.put<RequestWithClient>(`requests/${id}/note`, { note }))) setNote(null);
  }

  if (!req) {
    return error ? <Alert>{error}</Alert> : <Loading />;
  }

  const c = req.client;
  const noteValue = note ?? req.note;
  return (
    <>
      <Link href="/requests" className="text-sm text-zinc-600 hover:text-zinc-900">
        ← Все заявки
      </Link>
      <PageTitle actions={<StatusBadge status={req.status} />}>
        Заявка №{req.number} · {req.service_name}
      </PageTitle>

      {error && (
        <div className="mb-4">
          <Alert>{error}</Alert>
        </div>
      )}

      <div className="grid items-start gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card title="Что нужно клиенту">
            <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-zinc-500">Услуга</dt>
                <dd className="font-medium">{req.service_name}</dd>
              </div>
              {req.answers.map((a, i) => (
                <div key={i}>
                  <dt className="text-zinc-500">{a.label}</dt>
                  <dd className="whitespace-pre-wrap font-medium">{a.value}</dd>
                </div>
              ))}
            </dl>
            {req.message && (
              <figure className="mt-5 border-l-2 border-zinc-200 pl-4 text-sm">
                <figcaption className="text-zinc-500">Первое сообщение клиента</figcaption>
                <blockquote className="mt-1 whitespace-pre-wrap text-zinc-700">{req.message}</blockquote>
              </figure>
            )}
            <p className="mt-5 text-xs text-zinc-500">
              Создана {formatDate(req.created_at)} · обновлена {formatDate(req.updated_at)}
            </p>
          </Card>

          <Card title="Действия">
            {req.status === "new" && (
              <div className="flex flex-wrap gap-2">
                <Button variant="success" disabled={busy} onClick={() => act("take")}>
                  Взять в работу
                </Button>
                <Button variant="danger" disabled={busy} onClick={() => act("reject")}>
                  Отклонить
                </Button>
              </div>
            )}
            {req.status === "in_progress" && (
              <Button disabled={busy} onClick={() => act("close")}>
                Закрыть заявку
              </Button>
            )}
            {req.status !== "new" && req.status !== "in_progress" && (
              <p className="text-sm text-zinc-500">Заявка в финальном статусе — действий нет.</p>
            )}
          </Card>

          <Card title="Заметка мастера">
            <textarea
              value={noteValue}
              onChange={(e) => setNote(e.target.value)}
              rows={4}
              maxLength={4000}
              placeholder="Например: договорились на субботу, 11:00; нужно заказать фильтр. Клиент заметку не видит."
              className="w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm outline-none focus:border-zinc-900 focus:ring-1 focus:ring-zinc-900"
            />
            <div className="mt-3">
              <Button disabled={busy || note === null || note === req.note} onClick={saveNote}>
                Сохранить заметку
              </Button>
            </div>
          </Card>
        </div>

        <Card title="Клиент">
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="text-zinc-500">Имя</dt>
              <dd className="font-medium">{clientName(c)}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Телефон для связи</dt>
              <dd>
                <a className="text-base font-semibold underline" href={telHref(req.phone)}>
                  {formatPhone(req.phone)}
                </a>
              </dd>
            </div>
            <div>
              <dt className="text-zinc-500">Telegram</dt>
              <dd>
                {c.telegram_username ? (
                  <a className="underline" href={`https://t.me/${c.telegram_username}`} target="_blank" rel="noreferrer">
                    @{c.telegram_username}
                  </a>
                ) : (
                  <span>ID {c.telegram_id}</span>
                )}
              </dd>
            </div>
          </dl>
          <Link href={`/clients/${c.id}`} className="mt-4 inline-block text-sm font-medium underline">
            Все заявки клиента →
          </Link>
        </Card>
      </div>
    </>
  );
}
