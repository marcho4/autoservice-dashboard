"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import ConfirmButton from "@/components/ConfirmButton";
import { Alert, Button, CallButton, Card, Fact, linkClass, Loading, PageTitle, StatusBadge, TextArea, When } from "@/components/ui";
import { api, clientName, errorText, RequestWithClient, STATUS_LABELS } from "@/lib/api";

type Action = "take" | "reject" | "close" | "note";

export default function RequestPage() {
  const { id } = useParams<{ id: string }>();
  const [req, setReq] = useState<RequestWithClient | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<Action | "">("");
  const [note, setNote] = useState<string | null>(null);
  const [noteSaved, setNoteSaved] = useState(false);

  useEffect(() => {
    api.get<RequestWithClient>(`requests/${id}`).then(setReq).catch((e) => setError(errorText(e)));
  }, [id]);

  async function run(action: Action, call: () => Promise<RequestWithClient>) {
    setBusy(action);
    setError("");
    try {
      setReq(await call());
      return true;
    } catch (e) {
      setError(errorText(e));
      return false;
    } finally {
      setBusy("");
    }
  }

  const act = (action: "take" | "reject" | "close") => run(action, () => api.post<RequestWithClient>(`requests/${id}/${action}`));

  async function saveNote() {
    const submittedNote = note;
    if (await run("note", () => api.put<RequestWithClient>(`requests/${id}/note`, { note: submittedNote }))) {
      setNote((current) => (current === submittedNote ? null : current));
      setNoteSaved(true);
    }
  }

  if (!req) {
    return error ? <Alert>{error}</Alert> : <Loading />;
  }

  const c = req.client;
  const noteValue = note ?? req.note;
  return (
    <>
      <PageTitle
        back={{ href: "/requests", label: "Все заявки" }}
        lead={
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-meta">
            <StatusBadge status={req.status} />
            <span className="tabular-nums">Заявка №{req.number}</span>
            <span aria-hidden="true">·</span>
            <When iso={req.created_at} />
          </span>
        }
      >
        {req.service_name}
      </PageTitle>

      {error && (
        <div className="mb-4">
          <Alert>{error}</Alert>
        </div>
      )}

      <div className="grid items-start gap-4 lg:grid-cols-3 lg:gap-6">
        <Card className="lg:sticky lg:top-6 lg:col-start-3 lg:row-start-1">
          <h2 className="text-heading">{clientName(c)}</h2>
          <CallButton large phone={req.phone} name={clientName(c)} className="mt-3 w-full justify-start" />
          <p className="mt-2 flex flex-wrap items-center gap-x-4">
            {c.telegram_username && (
              <a className={linkClass} href={`https://t.me/${c.telegram_username}`} target="_blank" rel="noreferrer">
                @{c.telegram_username} в Telegram
              </a>
            )}
            <Link href={`/clients/${c.id}`} className={linkClass}>
              Все заявки клиента
            </Link>
          </p>

          <div className="mt-4 flex flex-wrap gap-2 border-t border-line pt-4">
            {req.status === "new" && (
              <>
                <Button variant="secondary" className="flex-1" disabled={busy !== ""} onClick={() => act("take")}>
                  {busy === "take" ? "Берём в работу…" : "Взять в работу"}
                </Button>
                <ConfirmButton label="Отклонить" confirmLabel="Да, отклонить заявку" disabled={busy !== ""} onConfirm={() => act("reject")} />
              </>
            )}
            {req.status === "in_progress" && (
              <Button variant="secondary" className="flex-1" disabled={busy !== ""} onClick={() => act("close")}>
                {busy === "close" ? "Закрываем…" : "Закрыть заявку"}
              </Button>
            )}
            {req.status !== "new" && req.status !== "in_progress" && (
              <p className="text-ink-2">Действий не требуется: заявка {STATUS_LABELS[req.status].toLowerCase()}.</p>
            )}
          </div>
        </Card>

        <div className="min-w-0 space-y-4 lg:col-span-2 lg:row-start-1 lg:space-y-6">
          <Card title="Что нужно клиенту">
            <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {req.answers.map((a, i) => (
                <Fact key={i} label={a.label}>
                  {a.value}
                </Fact>
              ))}
            </dl>
            {req.message && (
              <figure className="mt-5 border-l-2 border-line-strong pl-4">
                <figcaption className="text-meta text-ink-3">Первое сообщение клиента</figcaption>
                <blockquote className="mt-0.5 whitespace-pre-wrap">{req.message}</blockquote>
              </figure>
            )}
            {req.updated_at !== req.created_at && (
              <p className="mt-5 text-meta text-ink-3">
                Обновлена <When iso={req.updated_at} />
              </p>
            )}
          </Card>

          <Card>
            <TextArea
              label="Заметка мастера"
              hint="Клиент заметку не видит."
              value={noteValue}
              onChange={(e) => {
                setNote(e.target.value);
                setNoteSaved(false);
              }}
              rows={4}
              maxLength={4000}
              placeholder="Например: договорились на субботу, 11:00; нужно заказать фильтр."
            />
            <div className="mt-3 flex flex-wrap items-center gap-3">
              <Button variant="secondary" disabled={busy !== "" || note === null || note === req.note} onClick={saveNote}>
                {busy === "note" ? "Сохраняем…" : "Сохранить заметку"}
              </Button>
              <span role="status" className="font-medium text-ok">
                {noteSaved && note === null && "Заметка сохранена"}
              </span>
            </div>
          </Card>
        </div>
      </div>
    </>
  );
}
