import "server-only";
import { NextResponse } from "next/server";

export const SESSION_COOKIE = "session";

// Read at request time, so one image works in every environment.
export function backendUrl(): string {
  const url = process.env.BACKEND_URL;
  if (!url) throw new Error("BACKEND_URL is not set");
  return url.replace(/\/+$/, "");
}

type Session = { access_token: string; expires_at: string };

export function setSessionCookie(res: NextResponse, session: Session) {
  res.cookies.set(SESSION_COOKIE, session.access_token, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.COOKIE_SECURE === "true",
    path: "/",
    expires: new Date(session.expires_at),
  });
}

export function clearSessionCookie(res: NextResponse) {
  res.cookies.set(SESSION_COOKIE, "", { httpOnly: true, path: "/", maxAge: 0 });
}

export function backendUnavailable() {
  return NextResponse.json(
    { error: { code: "backend_unavailable", message: "backend is unavailable" } },
    { status: 502 },
  );
}
