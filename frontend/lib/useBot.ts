"use client";

import { useCallback, useEffect, useState } from "react";
import { api, ApiError, Bot } from "@/lib/api";

export function useBot(intervalMs = 5000) {
  const [bot, setBot] = useState<Bot | null | undefined>(undefined);
  const [error, setError] = useState<unknown>(null);

  const reload = useCallback(
    () =>
      api
        .get<Bot>("autoservice/bot")
        .then((b) => {
          setBot(b);
          setError(null);
        })
        .catch((e) => {
          if (e instanceof ApiError && (e.code === "bot_not_connected" || e.code === "no_autoservice")) {
            setBot(null);
            setError(null);
          } else {
            setError(e);
          }
        }),
    [],
  );

  useEffect(() => {
    reload();
    const id = setInterval(() => {
      if (!document.hidden) reload();
    }, intervalMs);
    return () => clearInterval(id);
  }, [reload, intervalMs]);

  return { bot, setBot, error, reload };
}
