"use client";

import { useEffect, useState } from "react";
import NoAutoservice from "@/components/NoAutoservice";
import ServiceEditor from "@/components/ServiceEditor";
import { Alert, Badge, Button, Card, Empty, Icon, IconButton, Loading, PageTitle } from "@/components/ui";
import { api, ApiError, errorText, List, Service, ServiceInput } from "@/lib/api";

const NEW = "new";

export default function ServicesPage() {
  const [services, setServices] = useState<Service[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [editing, setEditing] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api
      .get<List<Service>>("services")
      .then((d) => setServices(d.items))
      .catch(setError);
  }, []);

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
      const input: ServiceInput = { name: s.name, description: s.description, is_active: !s.is_active, fields: s.fields };
      const updated = await api.put<Service>(`services/${s.id}`, input);
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
              Добавить услугу
            </Button>
          )
        }
        lead="Клиент пишет боту своими словами. Бот сам определяет услугу, берёт из сообщения всё, что уже сказано, и задаёт только недостающие вопросы из формы услуги."
      >
        Услуги
      </PageTitle>

      {error !== null && (
        <div className="mb-4">
          <Alert>{errorText(error)}</Alert>
        </div>
      )}
      {!services && error === null && <Loading />}

      <div className="space-y-2">
        {editing === NEW && (
          <Card title="Новая услуга" outline="ink">
            <ServiceEditor service={null} onSave={save(null)} onCancel={() => setEditing("")} />
          </Card>
        )}

        {services && list.length === 0 && editing !== NEW && (
          <Empty
            title="Услуг нет"
            actions={
              <>
                <Button onClick={addDefaults} disabled={busy}>
                  Добавить типовые услуги
                </Button>
                <Button variant="secondary" onClick={() => setEditing(NEW)}>
                  Создать свою
                </Button>
              </>
            }
          >
            Без услуг бот не может принимать заявки.
          </Empty>
        )}

        {list.map((s, i) =>
          editing === s.id ? (
            <Card key={s.id} outline="ink">
              <ServiceEditor service={s} onSave={save(s)} onDelete={remove(s)} onCancel={() => setEditing("")} />
            </Card>
          ) : (
            <Card key={s.id} outline={s.is_active ? "line" : "dashed"}>
              <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
                <div className="w-full sm:w-auto sm:min-w-0 sm:flex-1">
                  <h2 className={`flex flex-wrap items-center gap-2 text-heading ${s.is_active ? "" : "text-ink-2"}`}>
                    {s.name}
                    {!s.is_active && <Badge tone="neutral">Выключена</Badge>}
                  </h2>
                  {s.description && <p className="mt-0.5 text-ink-2">{s.description}</p>}
                  <p className="mt-2 text-meta text-ink-2">
                    <span className="text-ink-3">Бот спросит: </span>
                    {[...s.fields.map((f) => (f.required ? f.label : `${f.label} (необязательно)`)), "Телефон"].join(" · ")}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <IconButton label="Выше" disabled={busy || i === 0} onClick={() => move(i, -1)}>
                    <Icon name="up" />
                  </IconButton>
                  <IconButton label="Ниже" disabled={busy || i === list.length - 1} onClick={() => move(i, 1)}>
                    <Icon name="down" />
                  </IconButton>
                  <Button variant="secondary" size="sm" disabled={busy} onClick={() => toggle(s)}>
                    {s.is_active ? "Выключить" : "Включить"}
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => setEditing(s.id)}>
                    Изменить
                  </Button>
                </div>
              </div>
            </Card>
          ),
        )}
      </div>
    </>
  );
}
