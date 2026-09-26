"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { Alert, Card, Loading, PageTitle, StatusBadge } from "@/components/ui";
import { api, Client, errorText, formatDate, ServiceRequest } from "@/lib/api";

type ClientDetails = Client & { requests: ServiceRequest[] };

export default function ClientPage() {
  const { id } = useParams<{ id: string }>();
  const [client, setClient] = useState<ClientDetails | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api.get<ClientDetails>(`clients/${id}`).then(setClient).catch((e) => setError(errorText(e)));
  }, [id]);

  if (!client) return error ? <Alert>{error}</Alert> : <Loading />;

  return (
    <>
      <Link href="/clients" className="text-sm text-zinc-600 hover:text-zinc-900">
        ← Все клиенты
      </Link>
      <PageTitle>{client.name || "Клиент без имени"}</PageTitle>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card title="Контакты">
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="text-zinc-500">Телефон</dt>
              <dd className="font-medium">{client.phone || "—"}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Telegram</dt>
              <dd>{client.telegram_username ? `@${client.telegram_username}` : `ID ${client.telegram_id}`}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Клиент с</dt>
              <dd>{formatDate(client.created_at)}</dd>
            </div>
          </dl>
        </Card>

        <div className="lg:col-span-2">
          <Card title={`Заявки (${client.requests.length})`}>
            <ul className="divide-y divide-zinc-100">
              {client.requests.map((r) => (
                <li key={r.id}>
                  <Link href={`/requests/${r.id}`} className="flex items-center justify-between gap-4 py-3 hover:bg-zinc-50">
                    <div className="min-w-0">
                      <div className="font-medium">
                        {r.car_brand} {r.car_model}
                      </div>
                      <div className="truncate text-sm text-zinc-600">{r.description}</div>
                      <div className="text-xs text-zinc-500">{formatDate(r.created_at)}</div>
                    </div>
                    <StatusBadge status={r.status} />
                  </Link>
                </li>
              ))}
            </ul>
          </Card>
        </div>
      </div>
    </>
  );
}
