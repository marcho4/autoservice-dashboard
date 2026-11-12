"use client";

import { FormEvent, useState } from "react";
import { Alert, Button } from "@/components/ui";
import { api, Bot, errorText } from "@/lib/api";

// Token input shared by the onboarding and the settings page. The backend verifies the token with Telegram
// and the bot runner starts polling on its own, so there is nothing else to do after a successful submit.
export default function BotTokenForm({
  label = "Токен бота",
  submitLabel = "Подключить бота",
  onConnected,
  onCancel,
}: {
  label?: string;
  submitLabel?: string;
  onConnected: (bot: Bot) => void;
  onCancel?: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const token = String(new FormData(e.currentTarget).get("token")).trim();
    setBusy(true);
    setError("");
    try {
      onConnected(await api.put<Bot>("autoservice/bot", { token }));
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3">
      <label className="block">
        <span className="mb-1 block text-sm font-medium text-zinc-700">{label}</span>
        <input
          name="token"
          required
          autoComplete="off"
          spellCheck={false}
          placeholder="123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
          className="w-full rounded-md border border-zinc-300 bg-white px-3 py-2 font-mono text-sm outline-none focus:border-zinc-900 focus:ring-1 focus:ring-zinc-900"
        />
      </label>
      {error && <Alert>{error}</Alert>}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" disabled={busy}>
          {busy ? "Проверяем токен…" : submitLabel}
        </Button>
        {onCancel && (
          <Button type="button" variant="secondary" onClick={onCancel}>
            Отмена
          </Button>
        )}
      </div>
    </form>
  );
}
