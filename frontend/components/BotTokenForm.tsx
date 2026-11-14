"use client";

import { FormEvent, useState } from "react";
import { Actions, Alert, Button, Field } from "@/components/ui";
import { api, Bot, errorText } from "@/lib/api";

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
      <Field
        label={label}
        name="token"
        required
        autoComplete="off"
        autoCapitalize="off"
        spellCheck={false}
        placeholder="123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
        className="font-mono"
      />
      {error && <Alert>{error}</Alert>}
      <Actions>
        <Button type="submit" disabled={busy}>
          {busy ? "Проверяем токен…" : submitLabel}
        </Button>
        {onCancel && (
          <Button type="button" variant="ghost" onClick={onCancel}>
            Отмена
          </Button>
        )}
      </Actions>
    </form>
  );
}
