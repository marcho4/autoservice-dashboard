"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import NoAutoservice from "@/components/NoAutoservice";
import { Alert, Loading, PageTitle } from "@/components/ui";
import { api, ApiError, ClientSummary, errorText, formatDate, List } from "@/lib/api";

export default function ClientsPage() {
  const [data, setData] = useState<List<ClientSummary> | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    api.get<List<ClientSummary>>("clients").then(setData).catch(setError);
  }, []);

  if (error instanceof ApiError && error.code === "no_autoservice") {
    return (
      <>
        <PageTitle>Клиенты</PageTitle>
        <NoAutoservice />
      </>
    );
  }

  return (
    <>
      <PageTitle>Клиенты</PageTitle>
      {error !== null && <Alert>{errorText(error)}</Alert>}
      {!data && error === null && <Loading />}
      {data && (
        <div className="overflow-x-auto rounded-lg border border-zinc-200 bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase text-zinc-500">
              <tr>
                <th className="px-4 py-3 font-medium">Клиент</th>
                <th className="px-4 py-3 font-medium">Telegram</th>
                <th className="px-4 py-3 font-medium">Телефон</th>
                <th className="px-4 py-3 font-medium">Заявок</th>
                <th className="px-4 py-3 font-medium">Последняя заявка</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {data.items.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-10 text-center text-zinc-500">
                    Клиентов пока нет — они появятся с первой заявкой через бота
                  </td>
                </tr>
              )}
              {data.items.map((c) => (
                <tr key={c.id} className="hover:bg-zinc-50">
                  <td className="px-4 py-3 font-medium">
                    <Link href={`/clients/${c.id}`} className="hover:underline">
                      {c.name || "Без имени"}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-zinc-700">{c.telegram_username ? `@${c.telegram_username}` : c.telegram_id}</td>
                  <td className="whitespace-nowrap px-4 py-3">{c.phone || "—"}</td>
                  <td className="px-4 py-3">{c.requests_count}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-zinc-600">{c.last_request_at ? formatDate(c.last_request_at) : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
