"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { Alert, Button, Field } from "@/components/ui";
import { api, errorText } from "@/lib/api";

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
        await api.register(email, password, String(form.get("full_name")));
        router.replace("/service");
      } else {
        await api.login(email, password);
        router.replace("/requests");
      }
      router.refresh();
    } catch (err) {
      setError(errorText(err));
      setBusy(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <div className="text-3xl">🔧</div>
          <h1 className="mt-2 text-xl font-semibold">{isRegister ? "Регистрация сотрудника" : "Вход в дашборд"}</h1>
          <p className="mt-1 text-sm text-zinc-500">Заявки вашего автосервиса из Telegram</p>
        </div>
        <form onSubmit={onSubmit} className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 shadow-sm">
          {isRegister && <Field label="Имя" name="full_name" required autoComplete="name" />}
          <Field label="Email" name="email" type="email" required autoComplete="email" />
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
            {busy ? "Подождите…" : isRegister ? "Зарегистрироваться" : "Войти"}
          </Button>
        </form>
        <p className="mt-4 text-center text-sm text-zinc-600">
          {isRegister ? (
            <>
              Уже есть аккаунт? <Link className="font-medium text-zinc-900 underline" href="/login">Войти</Link>
            </>
          ) : (
            <>
              Нет аккаунта? <Link className="font-medium text-zinc-900 underline" href="/register">Зарегистрироваться</Link>
            </>
          )}
        </p>
      </div>
    </main>
  );
}
