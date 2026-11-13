"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { Alert, CallButton, Card, Fact, listClass, Loading, PageTitle, StatusBadge, When } from "@/components/ui";
import { answersDigest, api, Client, errorText, requestsCount, ServiceRequest } from "@/lib/api";

type ClientDetails = Client & { requests: ServiceRequest[] };

export default function ClientPage() {
  const { id } = useParams<{ id: string }>();
  const [client, setClient] = useState<ClientDetails | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api.get<ClientDetails>(`clients/${id}`).then(setClient).catch((e) => setError(errorText(e)));
  }, [id]);

  if (!client) return error ? <Alert>{error}</Alert> : <Loading />;

  const name = client.name || "Клиент без имени";
  return (
    <>
      <PageTitle back={{ href: "/clients", label: "Все клиенты" }}>{name}</PageTitle>

      <div className="grid items-start gap-4 lg:grid-cols-3 lg:gap-6">
        <Card>
          {client.phone && <CallButton large phone={client.phone} name={name} className="mb-4 w-full justify-start" />}
          <dl className="space-y-3">
            {!client.phone && <Fact label="Телефон">—</Fact>}
            <Fact label="Telegram">{client.telegram_username ? `@${client.telegram_username}` : "—"}</Fact>
            <Fact label="Первое обращение">
              <When iso={client.created_at} />
            </Fact>
          </dl>
        </Card>

        <div className="min-w-0 lg:col-span-2">
          <h2 className="mb-2 text-heading">{requestsCount(client.requests.length)}</h2>
          <ul className={listClass}>
            {client.requests.map((r) => (
              <li key={r.id}>
                <Link href={`/requests/${r.id}`} className="flex items-start justify-between gap-4 px-4 py-3 hover:bg-paper active:bg-paper sm:px-5">
                  <div className="min-w-0">
                    <div className="font-medium">
                      {r.service_name} <span className="text-meta font-normal text-ink-3 tabular-nums">№{r.number}</span>
                    </div>
                    <div className="line-clamp-2 text-ink-2 sm:line-clamp-1">{answersDigest(r)}</div>
                    <When iso={r.created_at} className="text-meta text-ink-3" />
                  </div>
                  <StatusBadge status={r.status} />
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </>
  );
}
