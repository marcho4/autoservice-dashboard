"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { Alert, Button, Card, Loading, PageTitle, StatusBadge } from "@/components/ui";
import { api, errorText, formatDate, RequestWithClient } from "@/lib/api";

export default function RequestPage() {
  const { id } = useParams<{ id: string }>();
  const [req, setReq] = useState<RequestWithClient | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.get<RequestWithClient>(`requests/${id}`).then(setReq).catch((e) => setError(errorText(e)));
  }, [id]);

  async function act(action: "take" | "reject" | "close") {
    setBusy(true);
    setError("");
    try {
      setReq(await api.post<RequestWithClient>(`requests/${id}/${action}`));
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  if (!req) {
    return error ? <Alert>{error}</Alert> : <Loading />;
  }

  const c = req.client;
  return (
    <>
      <Link href="/requests" className="text-sm text-zinc-600 hover:text-zinc-900">
        ← Все заявки
      </Link>
      <PageTitle actions={<StatusBadge status={req.status} />}>
        {req.car_brand} {req.car_model}
      </PageTitle>

      {error && (
        <div className="mb-4">
          <Alert>{error}</Alert>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card title="Заявка">
            <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-zinc-500">Марка</dt>
                <dd className="font-medium">{req.car_brand}</dd>
              </div>
              <div>
                <dt className="text-zinc-500">Модель</dt>
                <dd className="font-medium">{req.car_model}</dd>
              </div>
              <div className="sm:col-span-2">
                <dt className="text-zinc-500">Описание проблемы</dt>
                <dd className="whitespace-pre-wrap">{req.description}</dd>
              </div>
              <div>
                <dt className="text-zinc-500">Создана</dt>
                <dd>{formatDate(req.created_at)}</dd>
              </div>
              <div>
                <dt className="text-zinc-500">Обновлена</dt>
                <dd>{formatDate(req.updated_at)}</dd>
              </div>
            </dl>
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
        </div>

        <Card title="Клиент">
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="text-zinc-500">Имя</dt>
              <dd className="font-medium">{c.name || "—"}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Телефон для связи</dt>
              <dd>
                <a className="font-medium underline" href={`tel:${req.phone.replace(/[^+\d]/g, "")}`}>
                  {req.phone}
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
