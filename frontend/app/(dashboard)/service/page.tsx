"use client";

import { FormEvent, useEffect, useState } from "react";
import ConfirmButton from "@/components/ConfirmButton";
import { Alert, Button, Card, Field, Loading, PageTitle, TextArea } from "@/components/ui";
import { api, ApiError, Autoservice, Bot, errorText, formatDate } from "@/lib/api";

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
        setStatus({ kind: "success", text: "Автосервис зарегистрирован. Теперь подключите Telegram-бота." });
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
  const [bot, setBot] = useState<Bot | null | undefined>(undefined);
  const [editing, setEditing] = useState(false);
  const [status, setStatus] = useState<Status>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api
      .get<Bot>("autoservice/bot")
      .then(setBot)
      .catch((e) => {
        if (e instanceof ApiError && e.code === "bot_not_connected") setBot(null);
        else setStatus({ kind: "error", text: errorText(e) });
      });
  }, []);

  async function connect(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const token = String(new FormData(form).get("token"));
    setBusy(true);
    setStatus(null);
    try {
      const b = await api.put<Bot>("autoservice/bot", { token });
      setBot(b);
      setEditing(false);
      form.reset();
      setStatus({ kind: "success", text: `Бот @${b.username} подключён` });
    } catch (err) {
      setStatus({ kind: "error", text: errorText(err) });
    } finally {
      setBusy(false);
    }
  }

  async function disconnect() {
    setBusy(true);
    setStatus(null);
    try {
      await api.del("autoservice/bot");
      setBot(null);
      setStatus({ kind: "success", text: "Бот отключён" });
    } catch (err) {
      setStatus({ kind: "error", text: errorText(err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title="Telegram-бот">
      <div className="space-y-4">
        {status && <Alert kind={status.kind}>{status.text}</Alert>}
        {bot === undefined && !status && <Loading />}

        {bot && (
          <dl className="grid gap-4 text-sm sm:grid-cols-3">
            <div>
              <dt className="text-zinc-500">Бот</dt>
              <dd>
                <a className="font-medium underline" href={`https://t.me/${bot.username}`} target="_blank" rel="noreferrer">
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
            Создайте бота в <span className="font-medium">@BotFather</span> и вставьте его токен. Клиенты будут оставлять заявки через этого бота.
          </p>
        )}

        {(bot === null || editing) && (
          <form onSubmit={connect} className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1">
              <Field
                label={bot ? "Новый токен" : "Токен бота"}
                name="token"
                required
                autoComplete="off"
                placeholder="123456789:AAH…"
                className="w-full rounded-md border border-zinc-300 bg-white px-3 py-2 font-mono text-sm outline-none focus:border-zinc-900 focus:ring-1 focus:ring-zinc-900"
              />
            </div>
            <div className="flex gap-2">
              <Button type="submit" disabled={busy}>
                {busy ? "Проверяем…" : bot ? "Заменить" : "Подключить"}
              </Button>
              {editing && (
                <Button type="button" variant="secondary" onClick={() => setEditing(false)}>
                  Отмена
                </Button>
              )}
            </div>
          </form>
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
  );
}
