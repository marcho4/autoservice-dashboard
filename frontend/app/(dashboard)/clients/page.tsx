"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import NoAutoservice from "@/components/NoAutoservice";
import { Alert, CallButton, Empty, listClass, Loading, PageTitle, rowLinkClass, Table, When } from "@/components/ui";
import { api, ApiError, ClientSummary, errorText, formatPhone, List, requestsCount, telHref } from "@/lib/api";

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
      {data && data.items.length === 0 && <Empty title="Клиентов пока нет">Они появятся с первой заявкой через бота.</Empty>}

      {data && data.items.length > 0 && (
        <ul className={`sm:hidden ${listClass}`}>
          {data.items.map((c) => (
            <li key={c.id} className="relative p-4">
              <Link href={`/clients/${c.id}`} className={rowLinkClass}>
                {c.name || "Без имени"}
              </Link>
              <p className="mt-0.5 text-meta text-ink-2">
                {requestsCount(c.requests_count)}
                {c.last_request_at && (
                  <>
                    {" · последняя "}
                    <When iso={c.last_request_at} />
                  </>
                )}
              </p>
              {c.phone && <CallButton phone={c.phone} name={c.name || "клиент"} className="relative z-10 mt-3 w-full" />}
            </li>
          ))}
        </ul>
      )}

      {data && data.items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>Клиент</th>
              <th>Телефон</th>
              <th>Telegram</th>
              <th className="text-right">Заявок</th>
              <th>Последняя заявка</th>
            </tr>
          </thead>
          <tbody>
            {data.items.map((c) => (
              <tr key={c.id}>
                <td className="font-medium">
                  <Link href={`/clients/${c.id}`} className="hover:underline">
                    {c.name || "Без имени"}
                  </Link>
                </td>
                <td className="whitespace-nowrap">
                  {c.phone ? (
                    <a href={telHref(c.phone)} className="font-semibold tabular-nums hover:underline">
                      {formatPhone(c.phone)}
                    </a>
                  ) : (
                    "—"
                  )}
                </td>
                <td className="text-ink-2">{c.telegram_username ? `@${c.telegram_username}` : "—"}</td>
                <td className="text-right tabular-nums">{c.requests_count}</td>
                <td className="text-ink-2">{c.last_request_at ? <When iso={c.last_request_at} /> : "—"}</td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </>
  );
}
