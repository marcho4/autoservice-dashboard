"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { Alert, Button, Field, linkClass, Logo } from "@/components/ui";
import { api, errorText } from "@/lib/api";

function nextPath(): string {
  const next = new URLSearchParams(window.location.search).get("next") ?? "";
  if (!/^\/(?![/\\])/.test(next)) return "/requests";
  const url = new URL(next, window.location.origin);
  return url.origin === window.location.origin ? url.pathname + url.search : "/requests";
}

export default function AuthForm({ mode }: { mode: "login" | "register" }) {
  const router = useRouter();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const isRegister = mode === "register";

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const email = String(form.get("email"));
    const password = String(form.get("password"));
    setBusy(true);
    setError("");
    try {
      if (isRegister) {
        await api.register(email, password, String(form.get("full_name")), String(form.get("autoservice_name")));
        router.replace("/onboarding");
      } else {
        await api.login(email, password);
        router.replace(nextPath());
      }
      router.refresh();
    } catch (err) {
      setError(errorText(err));
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto min-h-dvh w-full max-w-104 px-4 pt-[12vh] pb-10">
      <Logo className="size-10" />
      <h1 className="mt-6 text-display">{isRegister ? "Подключите автосервис" : "Вход в кабинет"}</h1>
      <p className="mt-2 text-ink-2">
        {isRegister ? "Бот в Telegram принимает заявки, пока мастер занят машиной." : "Заявки вашего автосервиса из Telegram."}
      </p>
      <form method="post" onSubmit={onSubmit} className="mt-8 space-y-4">
        {isRegister && (
          <>
            <Field label="Название автосервиса" name="autoservice_name" required maxLength={200} placeholder="Гараж на Лесной" />
            <Field label="Ваше имя" name="full_name" required autoComplete="name" />
          </>
        )}
        <Field label="Email" name="email" type="email" required autoComplete="email" inputMode="email" />
        <Field
          label="Пароль"
          name="password"
          type="password"
          required
          minLength={isRegister ? 8 : undefined}
          autoComplete={isRegister ? "new-password" : "current-password"}
          hint={isRegister ? "Минимум 8 символов" : undefined}
        />
        {error && <Alert>{error}</Alert>}
        <Button type="submit" disabled={busy} className="w-full">
          {busy ? (isRegister ? "Создаём кабинет…" : "Входим…") : isRegister ? "Продолжить" : "Войти"}
        </Button>
      </form>
      <p className="mt-6 text-ink-2">
        {isRegister ? "Уже есть аккаунт? " : "Нет аккаунта? "}
        <Link className={linkClass} href={isRegister ? "/login" : "/register"}>
          {isRegister ? "Войти" : "Зарегистрироваться"}
        </Link>
      </p>
    </main>
  );
}
