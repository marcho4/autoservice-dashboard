"use client";

import { FormEvent, useEffect, useState } from "react";
import ConfirmButton from "@/components/ConfirmButton";
import { BotStatusBadge, botErrorText, clientInvite } from "@/components/BotStatus";
import BotTokenForm from "@/components/BotTokenForm";
import { Actions, Alert, Button, buttonClass, Card, CopyButton, Fact, Field, Loading, PageTitle, TextArea, textLinkClass, When } from "@/components/ui";
import { api, ApiError, Autoservice, errorText } from "@/lib/api";
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
      <PageTitle>{service ? "Автосервис и бот" : "Регистрация автосервиса"}</PageTitle>
      <div className="space-y-4 sm:space-y-6">
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
            <div className="sm:col-span-2">
              <Button type="submit" disabled={busy}>
                {service ? "Сохранить" : "Зарегистрировать"}
              </Button>
            </div>
          </form>
        </Card>

        {service && <BotCard key={service.id} serviceName={service.name} />}

        {/* Away from "Сохранить": deleting everything is not the neighbour of an everyday action. */}
        {service && (
          <Card title="Удаление автосервиса">
            <p className="mb-4 max-w-[65ch] text-ink-2">
              Автосервис будет удалён безвозвратно вместе с заявками, клиентами и подключённым ботом. Аккаунт останется.
            </p>
            <ConfirmButton label="Удалить автосервис" confirmLabel="Да, удалить всё: заявки, клиентов и бота" onConfirm={remove} disabled={busy} />
          </Card>
        )}
      </div>
    </>
  );
}

function BotCard({ serviceName }: { serviceName: string }) {
  const { bot, setBot, error, reload } = useBot(4000);
  const [editing, setEditing] = useState(false);
  const [status, setStatus] = useState<Status>(null);
  const [busy, setBusy] = useState(false);

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

  return (
    <>
      <Card title="Telegram-бот" actions={bot ? <BotStatusBadge bot={bot} /> : undefined}>
        <div className="space-y-4">
          {status && <Alert kind={status.kind}>{status.text}</Alert>}
          {error !== null && <Alert>{errorText(error)}</Alert>}
          {bot === undefined && error === null && <Loading />}
          {bot?.status === "error" && <Alert>{botErrorText(bot)}</Alert>}

          {bot && (
            <dl className="grid gap-4 sm:grid-cols-3">
              <Fact label="Бот">
                <a className={textLinkClass} href={bot.link} target="_blank" rel="noreferrer">
                  @{bot.username}
                </a>
              </Fact>
              <Fact label="Токен">
                <span className="font-mono">{bot.masked_token}</span>
              </Fact>
              <Fact label="Подключён">
                <When iso={bot.connected_at} />
              </Fact>
            </dl>
          )}

          {bot === null && (
            <p className="max-w-[65ch] text-ink-2">
              Создайте бота в{" "}
              <a className={textLinkClass} href="https://t.me/BotFather" target="_blank" rel="noreferrer">
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
            <Actions>
              <CopyButton text={clientInvite(serviceName, bot)}>Скопировать ссылку для клиентов</CopyButton>
              <Button variant="secondary" onClick={() => setEditing(true)}>
                Заменить токен
              </Button>
              <ConfirmButton label="Отключить бота" confirmLabel="Да, отключить" onConfirm={disconnect} disabled={busy} />
            </Actions>
          )}
        </div>
      </Card>

      {bot && (
        <Card title="Уведомления о новых заявках">
          <div className="space-y-4">
            <p className="max-w-[65ch] text-ink-2">
              Бот может присылать каждую новую заявку мастеру в Telegram. Откройте ссылку на телефоне мастера и нажмите
              «Запустить». Ссылка одноразовая: для следующего человека появится новая.
            </p>
            <Actions>
              <a href={bot.notify_link} target="_blank" rel="noreferrer" className={buttonClass({ variant: bot.recipients.length > 0 ? "secondary" : "primary" })}>
                Подключить уведомления
              </a>
              <CopyButton text={bot.notify_link}>Скопировать ссылку</CopyButton>
            </Actions>
            {bot.recipients.length === 0 ? (
              <p className="font-medium">Пока никто не подключён — заявки видны только в этом кабинете.</p>
            ) : (
              <ul className="divide-y divide-line rounded-control border border-line">
                {bot.recipients.map((r) => (
                  <li key={r.chat_id} className="flex flex-wrap items-center justify-between gap-x-3 py-1 pr-1 pl-3">
                    <span>
                      <span className="font-medium">{r.name || `Чат ${r.chat_id}`}</span>
                      <span className="text-ink-3">
                        {" "}
                        · подключён <When iso={r.created_at} />
                      </span>
                    </span>
                    <Button variant="danger" size="sm" disabled={busy} onClick={() => removeRecipient(r.chat_id)}>
                      Отключить
                    </Button>
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
