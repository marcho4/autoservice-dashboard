"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { BotStatusBadge, botErrorText } from "@/components/BotStatus";
import BotTokenForm from "@/components/BotTokenForm";
import { Alert, Button, Field, Loading } from "@/components/ui";
import { api, ApiError, Autoservice, errorText } from "@/lib/api";
import { useBot } from "@/lib/useBot";

const STEPS = ["Аккаунт", "Бот", "Готово"];

function Stepper({ current }: { current: number }) {
  return (
    <ol className="mb-8 flex items-center justify-center gap-2 text-sm">
      {STEPS.map((label, i) => (
        <li key={label} className="flex items-center gap-2">
          {i > 0 && <span className={`h-px w-6 sm:w-10 ${i <= current ? "bg-zinc-900" : "bg-zinc-300"}`} />}
          <span
            className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold ${
              i < current ? "bg-emerald-600 text-white" : i === current ? "bg-zinc-900 text-white" : "bg-zinc-200 text-zinc-500"
            }`}
          >
            {i < current ? "✓" : i + 1}
          </span>
          <span className={i === current ? "font-medium text-zinc-900" : "text-zinc-500"}>{label}</span>
        </li>
      ))}
    </ol>
  );
}

export default function OnboardingPage() {
  // undefined: loading, null: the account has no autoservice yet (registered before autoservices were part of sign-up).
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
  const step = !service ? 0 : bot ? 2 : 1;

  return (
    <main className="mx-auto min-h-screen max-w-xl px-4 py-10">
      <div className="mb-6 text-center">
        <div className="text-3xl">🔧</div>
        <h1 className="mt-2 text-xl font-semibold">{service ? service.name : "Настройка автосервиса"}</h1>
      </div>
      <Stepper current={loading ? 1 : step} />

      {error && (
        <div className="mb-4">
          <Alert>{error}</Alert>
        </div>
      )}
      {loading && !error && <Loading />}

      {!loading && service === null && (
        <form onSubmit={createService} className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 shadow-sm">
          <Field label="Название автосервиса" name="name" required maxLength={200} placeholder="Гараж на Лесной" />
          <Button type="submit" disabled={busy}>
            Продолжить
          </Button>
        </form>
      )}

      {!loading && service && bot === null && (
        <section className="rounded-lg border border-zinc-200 bg-white p-6 shadow-sm">
          <h2 className="text-lg font-semibold">Подключите Telegram-бота</h2>
          <p className="mt-1 text-sm text-zinc-600">
            Клиенты будут писать этому боту, а он — оформлять заявки. Создать бота можно за минуту:
          </p>
          <ol className="mt-4 list-decimal space-y-2 pl-5 text-sm text-zinc-700">
            <li>
              Откройте в Telegram{" "}
              <a className="font-medium text-zinc-900 underline" href="https://t.me/BotFather" target="_blank" rel="noreferrer">
                @BotFather
              </a>{" "}
              и отправьте команду <code className="rounded bg-zinc-100 px-1 py-0.5">/newbot</code>.
            </li>
            <li>Придумайте боту имя (например, название автосервиса) и адрес, который заканчивается на «bot».</li>
            <li>BotFather пришлёт токен — длинную строку с двоеточием. Скопируйте её и вставьте сюда.</li>
          </ol>
          <div className="mt-5">
            <BotTokenForm onConnected={setBot} />
          </div>
          <p className="mt-4 text-sm text-zinc-500">
            <Link href="/requests" className="underline">
              Сделаю позже
            </Link>
          </p>
        </section>
      )}

      {!loading && service && bot && (
        <section className="space-y-5 rounded-lg border border-zinc-200 bg-white p-6 shadow-sm">
          <div>
            <div className="flex flex-wrap items-center gap-3">
              <h2 className="text-lg font-semibold">Бот @{bot.username}</h2>
              <BotStatusBadge bot={bot} />
            </div>
            {bot.status === "pending" && <p className="mt-1 text-sm text-zinc-600">Запускаем бота, это займёт несколько секунд…</p>}
            {bot.status === "running" && (
              <p className="mt-1 text-sm text-zinc-600">Бот уже принимает заявки. Больше ничего настраивать не нужно.</p>
            )}
            {bot.status === "error" && (
              <div className="mt-3 space-y-3">
                <Alert>{botErrorText(bot)}</Alert>
                <BotTokenForm label="Новый токен" submitLabel="Заменить токен" onConnected={setBot} />
              </div>
            )}
          </div>

          <ul className="divide-y divide-zinc-100 border-y border-zinc-100 text-sm">
            <li className="flex flex-wrap items-center justify-between gap-3 py-4">
              <div className="min-w-0 flex-1">
                <div className="font-medium">Проверьте, как бот отвечает</div>
                <div className="text-zinc-600">Напишите ему как клиент, например: «Хочу поменять масло на Camry».</div>
              </div>
              <a
                href={bot.link}
                target="_blank"
                rel="noreferrer"
                className="rounded-md border border-zinc-300 px-3 py-1.5 font-medium hover:bg-zinc-50"
              >
                Открыть бота
              </a>
            </li>
            <li className="flex flex-wrap items-center justify-between gap-3 py-4">
              <div className="min-w-0 flex-1">
                <div className="font-medium">Получайте новые заявки в Telegram</div>
                {bot.recipients.length > 0 && <div className="font-medium text-emerald-700">✓ Уведомления подключены</div>}
                <div className="text-zinc-600">Откройте ссылку с телефона мастера и нажмите «Запустить» — бот будет присылать ему каждую заявку.</div>
              </div>
              <a
                href={bot.notify_link}
                target="_blank"
                rel="noreferrer"
                className="rounded-md border border-zinc-300 px-3 py-1.5 font-medium hover:bg-zinc-50"
              >
                {bot.recipients.length > 0 ? "Подключить ещё" : "Подключить уведомления"}
              </a>
            </li>
            <li className="flex flex-wrap items-center justify-between gap-3 py-4">
              <div className="min-w-0 flex-1">
                <div className="font-medium">Услуги и вопросы бота</div>
                <div className="text-zinc-600">Мы добавили типовые услуги автосервиса. Их и вопросы к ним можно изменить в любой момент.</div>
              </div>
              <Link href="/services" className="rounded-md border border-zinc-300 px-3 py-1.5 font-medium hover:bg-zinc-50">
                Настроить услуги
              </Link>
            </li>
          </ul>

          <Link href="/requests" className="inline-flex rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700">
            Перейти к заявкам
          </Link>
        </section>
      )}
    </main>
  );
}
