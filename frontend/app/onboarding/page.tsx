"use client";

import Link from "next/link";
import { FormEvent, ReactNode, useEffect, useState } from "react";
import { BotStatusBadge, botErrorText, clientInvite } from "@/components/BotStatus";
import BotTokenForm from "@/components/BotTokenForm";
import { Actions, Alert, Button, buttonClass, Card, CopyButton, Field, Icon, linkClass, Loading, Logo, textLinkClass } from "@/components/ui";
import { api, ApiError, Autoservice, errorText } from "@/lib/api";
import { useBot } from "@/lib/useBot";

const STEPS = ["Аккаунт", "Бот", "Уведомления"];

function Stepper({ current }: { current: number }) {
  return (
    <ol className="mb-6 flex flex-wrap items-center gap-x-2 gap-y-1 text-meta">
      {STEPS.map((label, i) => (
        <li key={label} aria-current={i === current ? "step" : undefined} className="flex items-center gap-2">
          {i > 0 && <span className="h-px w-4 bg-line-strong sm:w-8" />}
          <span
            className={`flex size-6 items-center justify-center rounded-full font-semibold tabular-nums ${
              i < current ? "bg-ok-soft text-ok" : i === current ? "bg-ink text-surface" : "bg-sunken text-ink-3"
            }`}
          >
            {i < current ? <Icon name="check" className="size-3.5" /> : i + 1}
          </span>
          <span className={i === current ? "font-semibold" : "text-ink-2"}>
            {label}
            {i < current && <span className="sr-only"> — готово</span>}
          </span>
        </li>
      ))}
    </ol>
  );
}

function Task({ title, children, actions }: { title: ReactNode; children: ReactNode; actions: ReactNode }) {
  return (
    <li className="py-4">
      <h3 className="font-semibold">{title}</h3>
      <p className="max-w-[65ch] text-ink-2">{children}</p>
      <div className="mt-3">
        <Actions>{actions}</Actions>
      </div>
    </li>
  );
}

export default function OnboardingPage() {
  const [service, setService] = useState<Autoservice | null | undefined>(undefined);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const { bot, setBot } = useBot(2500);

  useEffect(() => {
    api
      .get<Autoservice>("autoservice")
      .then(setService)
      .catch((e) => {
        if (e instanceof ApiError && e.code === "no_autoservice") setService(null);
        else setError(errorText(e));
      });
  }, []);

  async function createService(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      setService(await api.post<Autoservice>("autoservice", { name: String(new FormData(e.currentTarget).get("name")) }));
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }

  const loading = service === undefined || (service !== null && bot === undefined);
  const notified = Boolean(bot && bot.recipients.length > 0);
  const step = !service ? 0 : !bot || bot.status === "error" ? 1 : notified ? 3 : 2;

  return (
    <main className="mx-auto min-h-dvh max-w-2xl px-4 pt-8 pb-12 sm:pt-12">
      <div className="mb-6 flex items-center gap-3">
        <Logo className="size-9" />
        <h1 className="min-w-0 text-title">{service ? service.name : "Настройка автосервиса"}</h1>
      </div>
      <Stepper current={loading ? 1 : step} />

      {error && (
        <div className="mb-4">
          <Alert>{error}</Alert>
        </div>
      )}
      {loading && !error && <Loading />}

      {!loading && service === null && (
        <Card>
          <form onSubmit={createService} className="space-y-4">
            <Field label="Название автосервиса" name="name" required maxLength={200} placeholder="Гараж на Лесной" />
            <Button type="submit" disabled={busy}>
              Продолжить
            </Button>
          </form>
        </Card>
      )}

      {!loading && service && bot === null && (
        <Card>
          <h2 className="text-heading">Подключите Telegram-бота</h2>
          <p className="mt-1 text-ink-2">Клиенты будут писать этому боту, а он — оформлять заявки. Создать бота можно за минуту:</p>
          <ol className="mt-4 list-decimal space-y-2 pl-5 marker:font-semibold marker:text-ink-3">
            <li>
              Откройте в Telegram{" "}
              <a className={textLinkClass} href="https://t.me/BotFather" target="_blank" rel="noreferrer">
                @BotFather
              </a>{" "}
              и отправьте команду <code className="rounded-tag bg-sunken px-1.5 py-0.5 font-mono text-meta">/newbot</code>.
            </li>
            <li>Придумайте боту имя (например, название автосервиса) и адрес, который заканчивается на «bot».</li>
            <li>BotFather пришлёт токен — длинную строку с двоеточием. Скопируйте её и вставьте сюда.</li>
          </ol>
          <div className="mt-5">
            <BotTokenForm onConnected={setBot} />
          </div>
          <Link href="/requests" className={`mt-3 text-ink-2 ${linkClass}`}>
            Сделаю позже
          </Link>
        </Card>
      )}

      {!loading && service && bot && (
        <Card>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <h2 className="text-heading">Бот @{bot.username}</h2>
            <BotStatusBadge bot={bot} />
          </div>
          {bot.status === "pending" && <p className="mt-1 text-ink-2">Запускаем бота, это займёт несколько секунд…</p>}
          {bot.status === "running" && (
            <p className="mt-1 text-ink-2">
              {notified
                ? "Бот принимает заявки и присылает их в Telegram. Больше ничего настраивать не нужно."
                : "Бот уже принимает заявки. Остался один шаг: чтобы новые заявки приходили вам в Telegram, нажмите «Получать заявки в Telegram», а затем «Запустить» в боте."}
            </p>
          )}
          {bot.status === "error" && (
            <div className="mt-3 space-y-3">
              <Alert>{botErrorText(bot)}</Alert>
              <BotTokenForm label="Новый токен" submitLabel="Заменить токен" onConnected={setBot} />
            </div>
          )}

          <ul className="mt-4 divide-y divide-line border-y border-line">
            <Task
              title={
                notified ? (
                  <span className="inline-flex items-center gap-1.5 text-ok">
                    <Icon name="check" /> Уведомления подключены
                  </span>
                ) : (
                  "Получайте новые заявки в Telegram"
                )
              }
              actions={
                <>
                  <a href={bot.notify_link} target="_blank" rel="noreferrer" className={buttonClass({ variant: notified ? "secondary" : "primary" })}>
                    {notified ? "Подключить ещё" : "Получать заявки в Telegram"}
                  </a>
                  <CopyButton text={bot.notify_link}>Скопировать ссылку</CopyButton>
                </>
              }
            >
              Откройте ссылку с телефона мастера и нажмите «Запустить» — бот будет присылать ему каждую заявку. Если заявки
              должен получать другой человек, отправьте ему эту ссылку.
            </Task>
            <Task
              title="Расскажите клиентам о боте"
              actions={
                <>
                  <CopyButton text={clientInvite(service.name, bot)}>Скопировать ссылку для клиентов</CopyButton>
                  <a href={bot.link} target="_blank" rel="noreferrer" className={buttonClass({ variant: "secondary" })}>
                    Открыть бота
                  </a>
                </>
              }
            >
              Отправьте ссылку клиентам или проверьте бота сами: напишите ему, например, «Хочу поменять масло на Camry».
            </Task>
            <Task
              title="Услуги и вопросы бота"
              actions={
                <Link href="/services" className={buttonClass({ variant: "secondary" })}>
                  Настроить услуги
                </Link>
              }
            >
              Мы добавили типовые услуги автосервиса. Их и вопросы к ним можно изменить в любой момент.
            </Task>
          </ul>

          <Link href="/requests" className={buttonClass({ variant: notified ? "primary" : "secondary" }, "mt-4")}>
            Перейти к заявкам
          </Link>
        </Card>
      )}
    </main>
  );
}
