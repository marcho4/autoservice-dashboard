"use client";

import { useRouter } from "next/navigation";
import { FormEvent, useEffect, useState } from "react";
import { Alert, Button, Card, Field, Loading, PageTitle } from "@/components/ui";
import { api, Employee, errorText, formatDate } from "@/lib/api";

type Status = { kind: "error" | "success"; text: string } | null;

export default function AccountPage() {
  const router = useRouter();
  const [me, setMe] = useState<Employee | null>(null);
  const [profileStatus, setProfileStatus] = useState<Status>(null);
  const [passwordStatus, setPasswordStatus] = useState<Status>(null);
  const [deleteStatus, setDeleteStatus] = useState<Status>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.get<Employee>("me").then(setMe).catch((e) => setProfileStatus({ kind: "error", text: errorText(e) }));
  }, []);

  async function run(action: () => Promise<void>, setStatus: (s: Status) => void) {
    setBusy(true);
    setStatus(null);
    try {
      await action();
    } catch (err) {
      setStatus({ kind: "error", text: errorText(err) });
    } finally {
      setBusy(false);
    }
  }

  function saveProfile(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    run(async () => {
      setMe(await api.patch<Employee>("me", { full_name: f.get("full_name"), email: f.get("email") }));
      setProfileStatus({ kind: "success", text: "Данные сохранены" });
      router.refresh();
    }, setProfileStatus);
  }

  function changePassword(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const f = new FormData(form);
    if (f.get("new_password") !== f.get("repeat_password")) {
      setPasswordStatus({ kind: "error", text: "Новые пароли не совпадают" });
      return;
    }
    run(async () => {
      await api.put("me/password", { current_password: f.get("current_password"), new_password: f.get("new_password") });
      form.reset();
      setPasswordStatus({ kind: "success", text: "Пароль изменён. Другие сессии завершены." });
    }, setPasswordStatus);
  }

  function deleteAccount(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    run(async () => {
      await api.del("me", { password: f.get("password") });
      router.replace("/register");
      router.refresh();
    }, setDeleteStatus);
  }

  if (!me) return profileStatus ? <Alert>{profileStatus.text}</Alert> : <Loading />;

  return (
    <>
      <PageTitle>Аккаунт</PageTitle>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card title="Профиль">
          <form key={me.updated_at} onSubmit={saveProfile} className="space-y-4">
            <Field label="Имя" name="full_name" required defaultValue={me.full_name} />
            <Field label="Email" name="email" type="email" required defaultValue={me.email} />
            <p className="text-xs text-zinc-500">Зарегистрирован {formatDate(me.created_at)}</p>
            {profileStatus && <Alert kind={profileStatus.kind}>{profileStatus.text}</Alert>}
            <Button type="submit" disabled={busy}>
              Сохранить
            </Button>
          </form>
        </Card>

        <Card title="Смена пароля">
          <form onSubmit={changePassword} className="space-y-4">
            <Field label="Текущий пароль" name="current_password" type="password" required autoComplete="current-password" />
            <Field label="Новый пароль" name="new_password" type="password" required minLength={8} autoComplete="new-password" />
            <Field label="Повторите новый пароль" name="repeat_password" type="password" required minLength={8} autoComplete="new-password" />
            {passwordStatus && <Alert kind={passwordStatus.kind}>{passwordStatus.text}</Alert>}
            <Button type="submit" disabled={busy}>
              Сменить пароль
            </Button>
          </form>
        </Card>

        <div className="lg:col-span-2">
          <Card title="Удаление аккаунта">
            <form onSubmit={deleteAccount} className="space-y-4">
              <p className="text-sm text-zinc-600">
                Аккаунт будет удалён безвозвратно вместе с автосервисом, его заявками, клиентами и подключённым ботом.
              </p>
              <div className="max-w-sm">
                <Field label="Введите пароль для подтверждения" name="password" type="password" required autoComplete="current-password" />
              </div>
              {deleteStatus && <Alert kind={deleteStatus.kind}>{deleteStatus.text}</Alert>}
              <Button type="submit" variant="danger" disabled={busy}>
                Удалить аккаунт
              </Button>
            </form>
          </Card>
        </div>
      </div>
    </>
  );
}
