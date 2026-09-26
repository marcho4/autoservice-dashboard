"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import NoAutoservice from "@/components/NoAutoservice";
import { Alert, Button, Loading, PageTitle, StatusBadge } from "@/components/ui";
import { api, ApiError, errorText, formatDate, List, RequestStatus, RequestWithClient, STATUS_LABELS } from "@/lib/api";

const PAGE_SIZE = 20;
const FILTERS: (RequestStatus | "")[] = ["", "new", "in_progress", "closed", "rejected", "cancelled"];

export default function RequestsPage() {
  const [status, setStatus] = useState<RequestStatus | "">("");
  const [sort, setSort] = useState<"desc" | "asc">("desc");
  const [page, setPage] = useState(0);
  const [data, setData] = useState<List<RequestWithClient> | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [reload, setReload] = useState(0);

  useEffect(() => {
    const q = new URLSearchParams({ sort, limit: String(PAGE_SIZE), offset: String(page * PAGE_SIZE) });
    if (status) q.set("status", status);
    api
      .get<List<RequestWithClient>>(`requests?${q}`)
      .then((d) => {
        setData(d);
        setError(null);
      })
      .catch(setError);
  }, [status, sort, page, reload]);

  if (error instanceof ApiError && error.code === "no_autoservice") {
    return (
      <>
        <PageTitle>Заявки</PageTitle>
        <NoAutoservice />
      </>
    );
  }

  const pages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1;

  return (
    <>
      <PageTitle
        actions={
          <Button variant="secondary" onClick={() => setReload((n) => n + 1)}>
            Обновить
          </Button>
        }
      >
        Заявки
      </PageTitle>

      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap gap-1">
          {FILTERS.map((f) => (
            <button
              key={f || "all"}
              onClick={() => {
                setStatus(f);
                setPage(0);
              }}
              className={`rounded-full px-3 py-1 text-sm ${status === f ? "bg-zinc-900 text-white" : "bg-white text-zinc-700 ring-1 ring-zinc-200 hover:bg-zinc-100"}`}
            >
              {f ? STATUS_LABELS[f] : "Все"}
            </button>
          ))}
        </div>
        <button
          onClick={() => {
            setSort(sort === "desc" ? "asc" : "desc");
            setPage(0);
          }}
          className="text-sm text-zinc-700 hover:text-zinc-900"
        >
          Дата создания {sort === "desc" ? "↓ сначала новые" : "↑ сначала старые"}
        </button>
      </div>

      {error !== null && <Alert>{errorText(error)}</Alert>}
      {!data && error === null && <Loading />}
      {data && (
        <div className="overflow-x-auto rounded-lg border border-zinc-200 bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase text-zinc-500">
              <tr>
                <th className="px-4 py-3 font-medium">Создана</th>
                <th className="px-4 py-3 font-medium">Автомобиль</th>
                <th className="px-4 py-3 font-medium">Проблема</th>
                <th className="px-4 py-3 font-medium">Клиент</th>
                <th className="px-4 py-3 font-medium">Статус</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {data.items.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-10 text-center text-zinc-500">
                    Заявок пока нет
                  </td>
                </tr>
              )}
              {data.items.map((r) => (
                <tr key={r.id} className="hover:bg-zinc-50">
                  <td className="whitespace-nowrap px-4 py-3 text-zinc-600">
                    <Link href={`/requests/${r.id}`} className="hover:underline">
                      {formatDate(r.created_at)}
                    </Link>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 font-medium">
                    <Link href={`/requests/${r.id}`} className="hover:underline">
                      {r.car_brand} {r.car_model}
                    </Link>
                  </td>
                  <td className="max-w-xs truncate px-4 py-3 text-zinc-700">{r.description}</td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <Link href={`/clients/${r.client.id}`} className="hover:underline">
                      {r.client.name || (r.client.telegram_username ? `@${r.client.telegram_username}` : r.client.telegram_id)}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={r.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {data && data.total > PAGE_SIZE && (
        <div className="mt-4 flex items-center justify-between text-sm">
          <span className="text-zinc-600">
            Всего: {data.total} · страница {page + 1} из {pages}
          </span>
          <div className="flex gap-2">
            <Button variant="secondary" disabled={page === 0} onClick={() => setPage(page - 1)}>
              Назад
            </Button>
            <Button variant="secondary" disabled={page + 1 >= pages} onClick={() => setPage(page + 1)}>
              Вперёд
            </Button>
          </div>
        </div>
      )}
    </>
  );
}
