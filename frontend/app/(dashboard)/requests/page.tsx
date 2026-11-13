"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import NoAutoservice from "@/components/NoAutoservice";
import { Alert, Button, CallButton, Empty, Icon, listClass, Loading, PageTitle, rowLinkClass, StatusBadge, Table, When } from "@/components/ui";
import {
  answersDigest,
  api,
  ApiError,
  clientName,
  errorText,
  formatPhone,
  RequestPage,
  RequestStatus,
  telHref,
} from "@/lib/api";

const PAGE_SIZE = 20;
const REFRESH_MS = 15000;
// A filter names a group of requests, so the labels are plural, unlike the status of a single request.
const FILTERS: { status: RequestStatus | ""; label: string }[] = [
  { status: "", label: "Все" },
  { status: "new", label: "Новые" },
  { status: "in_progress", label: "В работе" },
  { status: "closed", label: "Закрытые" },
  { status: "rejected", label: "Отклонённые" },
  { status: "cancelled", label: "Отменённые" },
];

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

  return (
    <>
      <PageTitle
        actions={
          <>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setSort(sort === "desc" ? "asc" : "desc");
                setPage(0);
              }}
            >
              <Icon name={sort === "desc" ? "down" : "up"} />
              {sort === "desc" ? "Сначала новые" : "Сначала старые"}
            </Button>
            {/* The list refreshes itself every 15 seconds; on a phone the button would only push the requests down. */}
            <span className="hidden sm:block">
              <Button variant="secondary" size="sm" onClick={() => setReload((n) => n + 1)}>
                Обновить
              </Button>
            </span>
          </>
        }
      >
        Заявки
      </PageTitle>

      {/* One row that scrolls sideways on a phone, so the first request stays on the first screen. */}
      <div className="-mx-4 mb-4 flex gap-2 overflow-x-auto px-4 [scrollbar-width:none] sm:mx-0 sm:flex-wrap sm:px-0">
        {FILTERS.map((f) => {
          const count = data ? (f.status ? data.counts[f.status] : all) : 0;
          const active = status === f.status;
          return (
            <button
              key={f.status || "all"}
              aria-pressed={active}
              onClick={() => {
                setStatus(f.status);
                setPage(0);
              }}
              className={`flex min-h-control-sm shrink-0 items-center gap-2 rounded-control border px-3 text-meta font-medium whitespace-nowrap transition-colors duration-150 ${
                active ? "border-ink bg-ink text-surface" : "border-line-strong bg-surface text-ink-2 hover:text-ink active:bg-sunken"
              }`}
            >
              {f.label}
              {count > 0 && (
                <span
                  className={`rounded-tag px-1.5 font-semibold tabular-nums ${
                    f.status === "new" ? "bg-accent text-ink" : active ? "bg-surface/20" : "bg-sunken text-ink-2"
                  }`}
                >
                  {count}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {error !== null && (
        <div className="mb-4">
          <Alert>{errorText(error)}</Alert>
        </div>
      )}
      {!data && error === null && <Loading />}
      {data && data.items.length === 0 && (
        <Empty title={all === 0 ? "Заявок пока нет" : "В этом статусе заявок нет"}>
          {all === 0 && "Они появятся здесь, как только клиент напишет боту."}
        </Empty>
      )}

      {/* Phone: the whole card opens the request, the phone number is a button of its own. */}
      {data && data.items.length > 0 && (
        <ul className={`sm:hidden ${listClass}`}>
          {data.items.map((r) => (
            <li key={r.id} className="relative p-4">
              <div className="flex items-center gap-2 text-meta text-ink-3">
                <StatusBadge status={r.status} />
                <span className="tabular-nums">№{r.number}</span>
                <When iso={r.created_at} className="ml-auto" />
              </div>
              <Link href={`/requests/${r.id}`} className={`mt-2 ${rowLinkClass}`}>
                {r.service_name}
              </Link>
              <p className="mt-1 line-clamp-2 text-ink-2">{answersDigest(r)}</p>
              <p className="mt-2 font-medium">{clientName(r.client)}</p>
              <div className="relative z-10 mt-2 flex gap-2">
                <CallButton phone={r.phone} name={clientName(r.client)} className="flex-1" />
                {r.status === "new" && (
                  <Button variant="secondary" disabled={busyId === r.id} onClick={() => take(r.id)}>
                    {busyId === r.id ? "Берём…" : "В работу"}
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {/* Desktop: status first to scan the column, then what is asked and whom to call. */}
      {data && data.items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>Статус</th>
              <th>Заявка</th>
              <th>Клиент</th>
              <th>Создана</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {data.items.map((r) => (
              <tr key={r.id} onClick={() => router.push(`/requests/${r.id}`)} className="cursor-pointer">
                <td>
                  <StatusBadge status={r.status} />
                </td>
                <td className="max-w-sm">
                  <Link
                    href={`/requests/${r.id}`}
                    className={`hover:underline ${r.status === "new" ? "font-semibold" : "font-medium"}`}
                    onClick={(e) => e.stopPropagation()}
                  >
                    {r.service_name}
                  </Link>
                  <span className="ml-2 text-meta text-ink-3 tabular-nums">№{r.number}</span>
                  <div className="truncate text-ink-2">{answersDigest(r)}</div>
                </td>
                <td className="whitespace-nowrap">
                  <div className="text-ink-2">{clientName(r.client)}</div>
                  <a href={telHref(r.phone)} className="font-semibold tabular-nums hover:underline" onClick={(e) => e.stopPropagation()}>
                    {formatPhone(r.phone)}
                  </a>
                </td>
                <td className="text-ink-2">
                  <When iso={r.created_at} />
                </td>
                <td className="text-right">
                  {r.status === "new" && (
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={busyId === r.id}
                      onClick={(e) => {
                        e.stopPropagation();
                        take(r.id);
                      }}
                    >
                      {busyId === r.id ? "Берём…" : "В работу"}
                    </Button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}

      {data && data.total > PAGE_SIZE && (
        <div className="mt-4 flex flex-wrap items-center justify-between gap-2">
          <span className="text-meta text-ink-2 tabular-nums">
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
