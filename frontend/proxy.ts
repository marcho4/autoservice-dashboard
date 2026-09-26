import { NextRequest, NextResponse } from "next/server";

const PUBLIC_PAGES = ["/login", "/register"];

// Redirects guests to the login page and signed-in employees away from it.
// The token itself is verified by the backend on every API call.
export function proxy(req: NextRequest) {
  const hasSession = Boolean(req.cookies.get("session")?.value);
  const isPublic = PUBLIC_PAGES.includes(req.nextUrl.pathname);

  if (!hasSession && !isPublic) {
    return NextResponse.redirect(new URL("/login", req.url));
  }
  if (hasSession && (isPublic || req.nextUrl.pathname === "/")) {
    return NextResponse.redirect(new URL("/requests", req.url));
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|healthz).*)"],
};
