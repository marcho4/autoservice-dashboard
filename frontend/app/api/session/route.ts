import { NextRequest, NextResponse } from "next/server";
import {
  SESSION_COOKIE,
  backendUnavailable,
  backendUrl,
  clearSessionCookie,
  setSessionCookie,
} from "@/lib/server/backend";

// POST /api/session — log in (or register with ?register=1) and store the JWT in an httpOnly cookie.
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

// DELETE /api/session — revoke the token on the backend and drop the cookie.
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
