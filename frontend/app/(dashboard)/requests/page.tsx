"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import NoAutoservice from "@/components/NoAutoservice";
import { Alert, Button, Loading, PageTitle, StatusBadge } from "@/components/ui";
import {
  answersDigest,
  api,
  ApiError,
  clientName,
  errorText,
  formatDate,
  formatPhone,
  RequestPage,
  RequestStatus,
  STATUS_LABELS,
  telHref,
} from "@/lib/api";

const PAGE_SIZE = 20;
const REFRESH_MS = 15000;
const FILTERS: (RequestStatus | "")[] = ["", "new", "in_progress", "closed", "rejected", "cancelled"];

export default function RequestsPage() {
  const router = useRouter();
  const [status, setStatus] = useState<RequestStatus | "">("");
  const [sort, setSort] = useState<"desc" | "asc">("desc");
  const [page, setPage] = useState(0);
  const [data, setData] = useState<RequestPage | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [reload, setReload] = useState(0);
  const [busyId, setBusyId] = useState("");

  useEffect(() => {
    const q = new URLSearchParams({ sort, limit: String(PAGE_SIZE), offset: String(page * PAGE_SIZE) });
    if (status) q.set("status", status);
    api
      .get<RequestPage>(`requests?${q}`)
      .then((d) => {
        setData(d);
        setError(null);
      })
      .catch(setError);
  }, [status, sort, page, reload]);

  // Requests arrive from the bot at any moment: keep the list fresh while the tab is open.
  useEffect(() => {
    const id = setInterval(() => {
      if (!document.hidden) setReload((n) => n + 1);
    }, REFRESH_MS);
    return () => clearInterval(id);
  }, []);

  async function take(id: string) {
    setBusyId(id);
    try {
      await api.post(`requests/${id}/take`);
    } catch (e) {
      setError(e);
    } finally {
      setBusyId("");
      setReload((n) => n + 1);
    }
  }

  if (error instanceof ApiError && error.code === "no_autoservice") {
    return (
      <>
        <PageTitle>Заявки</PageTitle>
        <NoAutoservice />
      </>
    );
  }

  const pages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1;
  const all = data ? Object.values(data.counts).reduce((a, b) => a + b, 0) : 0;
  const emptyText = all === 0 ? "Заявок пока нет — они появятся здесь, как только клиент напишет боту" : "В этом статусе заявок нет";

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
          {FILTERS.map((f) => {
            const count = data ? (f ? data.counts[f] : all) : null;
            const active = status === f;
            return (
              <button
                key={f || "all"}
                onClick={() => {
                  setStatus(f);
                  setPage(0);
                }}
                className={`flex items-center gap-1.5 rounded-full px-3 py-1 text-sm ${active ? "bg-zinc-900 text-white" : "bg-white text-zinc-700 ring-1 ring-zinc-200 hover:bg-zinc-100"}`}
              >
                {f ? STATUS_LABELS[f] : "Все"}
                {count !== null && count > 0 && (
                  <span
                    className={`rounded-full px-1.5 text-xs font-medium tabular-nums ${
                      f === "new" ? "bg-blue-600 text-white" : active ? "bg-white/20" : "bg-zinc-100 text-zinc-600"
                    }`}
                  >
                    {count}
                  </span>
                )}
              </button>
            );
          })}
        </div>
        <button
          onClick={() => {
            setSort(sort === "desc" ? "asc" : "desc");
            setPage(0);
          }}
          className="text-sm text-zinc-700 hover:text-zinc-900"
        >
          {sort === "desc" ? "↓ Сначала новые" : "↑ Сначала старые"}
        </button>
      </div>

      {error !== null && (
        <div className="mb-4">
          <Alert>{errorText(error)}</Alert>
        </div>
      )}
      {!data && error === null && <Loading />}
      {/* Phone: cards with the phone number and the action one tap away. */}
      {data && (
        <ul className="space-y-3 sm:hidden">
          {data.items.length === 0 && (
            <li className="rounded-lg border border-zinc-200 bg-white px-4 py-10 text-center text-sm text-zinc-500">{emptyText}</li>
          )}
          {data.items.map((r) => (
            <li key={r.id} className="rounded-lg border border-zinc-200 bg-white p-4 text-sm shadow-sm">
              <div className="flex items-start justify-between gap-3">
                <Link href={`/requests/${r.id}`} className="min-w-0 font-medium">
                  <span className="text-zinc-500">№{r.number}</span> {r.service_name}
                </Link>
                <StatusBadge status={r.status} />
              </div>
              <Link href={`/requests/${r.id}`} className="mt-1 line-clamp-2 block text-zinc-600">
                {answersDigest(r)}
              </Link>
              <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
                <div>
                  <div>{clientName(r.client)}</div>
                  <a href={telHref(r.phone)} className="font-medium underline">
                    {formatPhone(r.phone)}
                  </a>
                  <div className="text-xs text-zinc-500">{formatDate(r.created_at)}</div>
                </div>
                {r.status === "new" && (
                  <Button variant="secondary" disabled={busyId === r.id} onClick={() => take(r.id)}>
                    В работу
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
      {data && (
        <div className="hidden overflow-x-auto rounded-lg border border-zinc-200 bg-white shadow-sm sm:block">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase text-zinc-500">
              <tr>
                <th className="px-4 py-3 font-medium">№</th>
                <th className="px-4 py-3 font-medium">Создана</th>
                <th className="px-4 py-3 font-medium">Заявка</th>
                <th className="px-4 py-3 font-medium">Клиент</th>
                <th className="px-4 py-3 font-medium">Статус</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {data.items.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-10 text-center text-zinc-500">
                    {emptyText}
                  </td>
                </tr>
              )}
              {data.items.map((r) => (
                <tr key={r.id} onClick={() => router.push(`/requests/${r.id}`)} className="cursor-pointer align-top hover:bg-zinc-50">
                  <td className="px-4 py-3 tabular-nums text-zinc-500">{r.number}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-zinc-600">{formatDate(r.created_at)}</td>
                  <td className="max-w-sm px-4 py-3">
                    <Link href={`/requests/${r.id}`} className="font-medium hover:underline" onClick={(e) => e.stopPropagation()}>
                      {r.service_name}
                    </Link>
                    <div className="truncate text-zinc-600">{answersDigest(r)}</div>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <div>{clientName(r.client)}</div>
                    <a href={telHref(r.phone)} className="text-zinc-600 hover:underline" onClick={(e) => e.stopPropagation()}>
                      {formatPhone(r.phone)}
                    </a>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={r.status} />
                  </td>
                  <td className="px-4 py-2 text-right">
                    {r.status === "new" && (
                      <Button
                        variant="secondary"
                        className="whitespace-nowrap !px-3 !py-1.5"
                        disabled={busyId === r.id}
                        onClick={(e) => {
                          e.stopPropagation();
                          take(r.id);
                        }}
                      >
                        В работу
                      </Button>
                    )}
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
