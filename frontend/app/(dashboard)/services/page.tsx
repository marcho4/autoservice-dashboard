"use client";

import { useEffect, useState } from "react";
import NoAutoservice from "@/components/NoAutoservice";
import ServiceEditor from "@/components/ServiceEditor";
import { Alert, Button, Loading, PageTitle } from "@/components/ui";
import { api, ApiError, errorText, List, Service, ServiceInput } from "@/lib/api";

const NEW = "new";

export default function ServicesPage() {
  const [services, setServices] = useState<Service[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  // ID of the service being edited, NEW for the "add service" form.
  const [editing, setEditing] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api
      .get<List<Service>>("services")
      .then((d) => setServices(d.items))
      .catch(setError);
  }, []);

  // Runs a catalog change and reports a failure as text, which the editor shows next to its buttons.
  async function change(call: () => Promise<Service[]>): Promise<string | null> {
    try {
      setServices(await call());
      setError(null);
      return null;
    } catch (e) {
      return errorText(e);
    }
  }

  async function quick(call: () => Promise<Service[]>) {
    setBusy(true);
    const failure = await change(call);
    if (failure) setError(new ApiError(0, "client", failure));
    setBusy(false);
  }

  const list = services ?? [];

  const save = (current: Service | null) => async (input: ServiceInput) => {
    const failure = await change(async () => {
      if (current) {
        const updated = await api.put<Service>(`services/${current.id}`, input);
        return list.map((s) => (s.id === updated.id ? updated : s));
      }
      return [...list, await api.post<Service>("services", input)];
    });
    if (!failure) setEditing("");
    return failure;
  };

  const remove = (current: Service) => async () => {
    const failure = await change(async () => {
      await api.del(`services/${current.id}`);
      return list.filter((s) => s.id !== current.id);
    });
    if (!failure) setEditing("");
    return failure;
  };

  const toggle = (s: Service) =>
    quick(async () => {
      const updated = await api.put<Service>(`services/${s.id}`, { ...s, is_active: !s.is_active });
      return list.map((x) => (x.id === updated.id ? updated : x));
    });

  const move = (index: number, delta: number) =>
    quick(async () => {
      const ids = list.map((s) => s.id);
      const [id] = ids.splice(index, 1);
      ids.splice(index + delta, 0, id);
      return (await api.put<List<Service>>("services/order", { ids })).items;
    });

  const addDefaults = () => quick(async () => (await api.post<List<Service>>("services/defaults")).items);

  if (error instanceof ApiError && error.code === "no_autoservice") {
    return (
      <>
        <PageTitle>Услуги</PageTitle>
        <NoAutoservice />
      </>
    );
  }

  return (
    <>
      <PageTitle
        actions={
          services &&
          editing !== NEW && (
            <Button onClick={() => setEditing(NEW)} disabled={list.length >= 30}>
              + Добавить услугу
            </Button>
          )
        }
      >
        Услуги
      </PageTitle>
      <p className="-mt-3 mb-6 max-w-2xl text-sm text-zinc-600">
        Клиент пишет боту своими словами. Бот сам определяет услугу, берёт из сообщения всё, что уже сказано, и задаёт
        только недостающие вопросы из формы услуги.
      </p>

      {error !== null && (
        <div className="mb-4">
          <Alert>{errorText(error)}</Alert>
        </div>
      )}
      {!services && error === null && <Loading />}

      <div className="space-y-3">
        {editing === NEW && (
          <section className="rounded-lg border border-zinc-900 bg-white p-5 shadow-sm">
            <h2 className="mb-4 text-base font-semibold">Новая услуга</h2>
            <ServiceEditor service={null} onSave={save(null)} onCancel={() => setEditing("")} />
          </section>
        )}

        {services && list.length === 0 && editing !== NEW && (
          <section className="rounded-lg border border-dashed border-zinc-300 bg-white p-8 text-center">
            <p className="text-sm text-zinc-600">Услуг нет, поэтому бот не может принимать заявки.</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2">
              <Button onClick={addDefaults} disabled={busy}>
                Добавить типовые услуги
              </Button>
              <Button variant="secondary" onClick={() => setEditing(NEW)}>
                Создать свою
              </Button>
            </div>
          </section>
        )}

        {list.map((s, i) =>
          editing === s.id ? (
            <section key={s.id} className="rounded-lg border border-zinc-900 bg-white p-5 shadow-sm">
              <ServiceEditor service={s} onSave={save(s)} onDelete={remove(s)} onCancel={() => setEditing("")} />
            </section>
          ) : (
            <section key={s.id} className={`rounded-lg border border-zinc-200 bg-white p-5 shadow-sm ${s.is_active ? "" : "opacity-60"}`}>
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="w-full sm:w-auto sm:min-w-0 sm:flex-1">
                  <h2 className="flex flex-wrap items-center gap-2 text-base font-semibold">
                    {s.name}
                    {!s.is_active && <span className="rounded-full bg-zinc-200 px-2 py-0.5 text-xs font-medium text-zinc-700">Выключена</span>}
                  </h2>
                  {s.description && <p className="mt-0.5 text-sm text-zinc-600">{s.description}</p>}
                </div>
                <div className="flex flex-wrap items-center gap-1">
                  <button
                    title="Выше"
                    aria-label="Выше"
                    disabled={busy || i === 0}
                    onClick={() => move(i, -1)}
                    className="h-9 w-9 rounded-md border border-zinc-300 text-sm hover:bg-zinc-100 disabled:cursor-not-allowed disabled:text-zinc-300"
                  >
                    ↑
                  </button>
                  <button
                    title="Ниже"
                    aria-label="Ниже"
                    disabled={busy || i === list.length - 1}
                    onClick={() => move(i, 1)}
                    className="h-9 w-9 rounded-md border border-zinc-300 text-sm hover:bg-zinc-100 disabled:cursor-not-allowed disabled:text-zinc-300"
                  >
                    ↓
                  </button>
                  <Button variant="secondary" disabled={busy} onClick={() => toggle(s)}>
                    {s.is_active ? "Выключить" : "Включить"}
                  </Button>
                  <Button variant="secondary" onClick={() => setEditing(s.id)}>
                    Изменить
                  </Button>
                </div>
              </div>
              <ul className="mt-3 flex flex-wrap gap-1.5 text-xs">
                {s.fields.map((f) => (
                  <li key={f.id} className="rounded-full bg-zinc-100 px-2.5 py-1 text-zinc-700">
                    {f.label}
                    {!f.required && <span className="text-zinc-400"> · необязательно</span>}
                  </li>
                ))}
                <li className="rounded-full bg-zinc-100 px-2.5 py-1 text-zinc-700">Телефон</li>
              </ul>
            </section>
          ),
        )}
      </div>
    </>
  );
}
