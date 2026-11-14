import { NextRequest, NextResponse } from "next/server";

const PUBLIC_PAGES = ["/login", "/register"];

export function proxy(req: NextRequest) {
  const hasSession = Boolean(req.cookies.get("session")?.value);
  const isPublic = PUBLIC_PAGES.includes(req.nextUrl.pathname);

  if (!hasSession && !isPublic) {
    const login = new URL("/login", req.url);
    const next = req.nextUrl.pathname + req.nextUrl.search;
    if (next !== "/") login.searchParams.set("next", next);
    return NextResponse.redirect(login);
  }
  if (hasSession && (isPublic || req.nextUrl.pathname === "/")) {
    return NextResponse.redirect(new URL("/requests", req.url));
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|healthz).*)"],
};
