import { NextRequest, NextResponse } from "next/server";
import {
  SESSION_COOKIE,
  backendUnavailable,
  backendUrl,
  clearSessionCookie,
  setSessionCookie,
} from "@/lib/server/backend";

// Proxies /api/backend/<path> to <BACKEND_URL>/api/v1/<path>, attaching the JWT from the cookie.
async function handler(req: NextRequest, ctx: RouteContext<"/api/backend/[...path]">) {
  const { path } = await ctx.params;
  const target = `${backendUrl()}/api/v1/${path.map(encodeURIComponent).join("/")}${req.nextUrl.search}`;
  const token = req.cookies.get(SESSION_COOKIE)?.value;

  const headers: Record<string, string> = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  const hasBody = req.method !== "GET" && req.method !== "HEAD";
  if (hasBody) headers["Content-Type"] = "application/json";

  let upstream: Response;
  try {
    upstream = await fetch(target, {
      method: req.method,
      headers,
      body: hasBody ? await req.text() : undefined,
      cache: "no-store",
    });
  } catch {
    return backendUnavailable();
  }

  const text = await upstream.text();
  const endpoint = path.join("/");

  // Password change re-issues the token: swap the cookie and hide the token from the browser.
  if (endpoint === "me/password" && upstream.ok) {
    const body = JSON.parse(text);
    const res = NextResponse.json(body.employee, { status: 200 });
    setSessionCookie(res, body);
    return res;
  }

  const res = new NextResponse(upstream.status === 204 ? null : text, {
    status: upstream.status,
    headers: { "Content-Type": upstream.headers.get("Content-Type") ?? "application/json" },
  });
  if (upstream.status === 401 || (endpoint === "me" && req.method === "DELETE" && upstream.ok)) {
    clearSessionCookie(res);
  }
  return res;
}

export { handler as GET, handler as POST, handler as PUT, handler as PATCH, handler as DELETE };
