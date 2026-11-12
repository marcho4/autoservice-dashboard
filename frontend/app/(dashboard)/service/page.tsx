"use client";

import { FormEvent, useEffect, useState } from "react";
import ConfirmButton from "@/components/ConfirmButton";
import { BotStatusBadge, botErrorText } from "@/components/BotStatus";
import BotTokenForm from "@/components/BotTokenForm";
import { Alert, Button, Card, Field, Loading, PageTitle, TextArea } from "@/components/ui";
import { api, ApiError, Autoservice, errorText, formatDate } from "@/lib/api";
import { useBot } from "@/lib/useBot";

type Status = { kind: "error" | "success"; text: string } | null;

export default function ServicePage() {
  const [service, setService] = useState<Autoservice | null | undefined>(undefined);
  const [status, setStatus] = useState<Status>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api
      .get<Autoservice>("autoservice")
      .then(setService)
      .catch((e) => {
        if (e instanceof ApiError && e.code === "no_autoservice") setService(null);
        else setStatus({ kind: "error", text: errorText(e) });
      });
  }, []);

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const body = {
      name: String(f.get("name")),
      address: String(f.get("address")),
      phone: String(f.get("phone")),
      description: String(f.get("description")),
    };
    setBusy(true);
    setStatus(null);
    try {
      if (service) {
        setService(await api.put<Autoservice>("autoservice", body));
        setStatus({ kind: "success", text: "Данные автосервиса сохранены" });
      } else {
        setService(await api.post<Autoservice>("autoservice", body));
        setStatus({ kind: "success", text: "Автосервис зарегистрирован, типовые услуги добавлены. Теперь подключите Telegram-бота." });
      }
    } catch (err) {
      setStatus({ kind: "error", text: errorText(err) });
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    setBusy(true);
    setStatus(null);
    try {
      await api.del("autoservice");
      setService(null);
      setStatus({ kind: "success", text: "Автосервис удалён вместе с заявками, клиентами и ботом" });
    } catch (err) {
      setStatus({ kind: "error", text: errorText(err) });
    } finally {
      setBusy(false);
    }
  }

  if (service === undefined) return status ? <Alert>{status.text}</Alert> : <Loading />;

  return (
    <>
      <PageTitle>{service ? "Мой автосервис" : "Регистрация автосервиса"}</PageTitle>
      <div className="space-y-6">
        {status && <Alert kind={status.kind}>{status.text}</Alert>}

        <Card title="Данные автосервиса">
          <form key={service?.updated_at ?? "new"} onSubmit={save} className="grid gap-4 sm:grid-cols-2">
            <div className="sm:col-span-2">
              <Field label="Название" name="name" required defaultValue={service?.name} />
            </div>
            <Field label="Адрес" name="address" defaultValue={service?.address} />
            <Field label="Телефон" name="phone" type="tel" defaultValue={service?.phone} placeholder="+7 900 000-00-00" />
            <div className="sm:col-span-2">
              <TextArea label="Описание" name="description" defaultValue={service?.description} />
            </div>
            <div className="flex flex-wrap items-center gap-2 sm:col-span-2">
              <Button type="submit" disabled={busy}>
                {service ? "Сохранить" : "Зарегистрировать"}
              </Button>
              {service && (
                <ConfirmButton
                  label="Удалить автосервис"
                  confirmLabel="Да, удалить всё: заявки, клиентов и бота"
                  onConfirm={remove}
                  disabled={busy}
                />
              )}
            </div>
          </form>
        </Card>

        {service && <BotCard key={service.id} />}
      </div>
    </>
  );
}

function BotCard() {
  const { bot, setBot, error, reload } = useBot(4000);
  const [editing, setEditing] = useState(false);
  const [status, setStatus] = useState<Status>(null);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);

  async function run(action: () => Promise<void>, success: string) {
    setBusy(true);
    setStatus(null);
    try {
      await action();
      setStatus({ kind: "success", text: success });
    } catch (err) {
      setStatus({ kind: "error", text: errorText(err) });
    } finally {
      setBusy(false);
    }
  }

  const disconnect = () =>
    run(async () => {
      await api.del("autoservice/bot");
      setBot(null);
    }, "Бот отключён");

  const removeRecipient = (chatId: number) =>
    run(async () => {
      await api.del(`autoservice/bot/recipients/${chatId}`);
      await reload();
    }, "Уведомления для этого чата отключены");

  async function copyLink(link: string) {
    await navigator.clipboard.writeText(link).catch(() => undefined);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <>
      <Card title="Telegram-бот" actions={bot ? <BotStatusBadge bot={bot} /> : undefined}>
        <div className="space-y-4">
          {status && <Alert kind={status.kind}>{status.text}</Alert>}
          {error !== null && <Alert>{errorText(error)}</Alert>}
          {bot === undefined && error === null && <Loading />}
          {bot?.status === "error" && <Alert>{botErrorText(bot)}</Alert>}

          {bot && (
            <dl className="grid gap-4 text-sm sm:grid-cols-3">
              <div>
                <dt className="text-zinc-500">Бот</dt>
                <dd>
                  <a className="font-medium underline" href={bot.link} target="_blank" rel="noreferrer">
                    @{bot.username}
                  </a>
                </dd>
              </div>
              <div>
                <dt className="text-zinc-500">Токен</dt>
                <dd className="font-mono">{bot.masked_token}</dd>
              </div>
              <div>
                <dt className="text-zinc-500">Подключён</dt>
                <dd>{formatDate(bot.connected_at)}</dd>
              </div>
            </dl>
          )}

          {bot === null && (
            <p className="text-sm text-zinc-600">
              Создайте бота в{" "}
              <a className="font-medium text-zinc-900 underline" href="https://t.me/BotFather" target="_blank" rel="noreferrer">
                @BotFather
              </a>{" "}
              командой /newbot и вставьте его токен. Бот запустится сам и начнёт принимать заявки.
            </p>
          )}

          {(bot === null || editing) && (
            <BotTokenForm
              label={bot ? "Новый токен" : "Токен бота"}
              submitLabel={bot ? "Заменить" : "Подключить бота"}
              onCancel={editing ? () => setEditing(false) : undefined}
              onConnected={(b) => {
                setBot(b);
                setEditing(false);
                setStatus({ kind: "success", text: `Бот @${b.username} подключён и запускается` });
              }}
            />
          )}

          {bot && !editing && (
            <div className="flex flex-wrap gap-2">
              <Button variant="secondary" onClick={() => setEditing(true)}>
                Заменить токен
              </Button>
              <ConfirmButton label="Отключить бота" confirmLabel="Да, отключить" onConfirm={disconnect} disabled={busy} />
            </div>
          )}
        </div>
      </Card>

      {bot && (
        <Card title="Уведомления о новых заявках">
          <div className="space-y-4 text-sm">
            <p className="text-zinc-600">
              Бот может присылать каждую новую заявку мастеру в Telegram. Откройте ссылку на телефоне мастера и нажмите
              «Запустить». Ссылка одноразовая: для следующего человека появится новая.
            </p>
            <div className="flex flex-wrap gap-2">
              <a
                href={bot.notify_link}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center rounded-md bg-zinc-900 px-4 py-2 font-medium text-white hover:bg-zinc-700"
              >
                Подключить мой Telegram
              </a>
              <Button variant="secondary" onClick={() => copyLink(bot.notify_link)}>
                {copied ? "Скопировано" : "Скопировать ссылку"}
              </Button>
            </div>
            {bot.recipients.length === 0 ? (
              <p className="text-zinc-500">Пока никто не подключён — заявки видны только в этом кабинете.</p>
            ) : (
              <ul className="divide-y divide-zinc-100 rounded-md border border-zinc-200">
                {bot.recipients.map((r) => (
                  <li key={r.chat_id} className="flex flex-wrap items-center justify-between gap-3 px-3 py-2">
                    <span>
                      <span className="font-medium">{r.name || `Чат ${r.chat_id}`}</span>
                      <span className="text-zinc-500"> · с {formatDate(r.created_at)}</span>
                    </span>
                    <button className="text-red-700 hover:underline" disabled={busy} onClick={() => removeRecipient(r.chat_id)}>
                      Отключить
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </Card>
      )}
    </>
  );
}
