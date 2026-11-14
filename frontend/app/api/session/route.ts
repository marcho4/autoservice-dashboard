import { NextRequest, NextResponse } from "next/server";
import {
  SESSION_COOKIE,
  backendUnavailable,
  backendUrl,
  clearSessionCookie,
  setSessionCookie,
} from "@/lib/server/backend";

export async function POST(req: NextRequest) {
  const path = req.nextUrl.searchParams.get("register") ? "register" : "login";
  let upstream: Response;
  try {
    upstream = await fetch(`${backendUrl()}/api/v1/auth/${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: await req.text(),
      cache: "no-store",
    });
  } catch {
    return backendUnavailable();
  }
  const body = await upstream.json().catch(() => ({}));
  if (!upstream.ok) return NextResponse.json(body, { status: upstream.status });

  const res = NextResponse.json({ employee: body.employee }, { status: upstream.status });
  setSessionCookie(res, body);
  return res;
}

export async function DELETE(req: NextRequest) {
  const token = req.cookies.get(SESSION_COOKIE)?.value;
  if (token) {
    await fetch(`${backendUrl()}/api/v1/auth/logout`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      cache: "no-store",
    }).catch(() => undefined);
  }
  const res = new NextResponse(null, { status: 204 });
  clearSessionCookie(res);
  return res;
}
