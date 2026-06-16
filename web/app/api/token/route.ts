import { NextRequest, NextResponse } from "next/server";
import { SignJWT } from "jose";

// Dev-only token endpoint: mints a short-lived JWT for a hardcoded test user so
// the demo needs no signup flow. The signing secret must match the gateway's
// JWT_SECRET. NOT for production — a real deployment issues tokens from an
// authenticated control plane.
const TEST_USERS = new Set(["alice", "bob", "carol"]);

export async function GET(req: NextRequest) {
  const user = req.nextUrl.searchParams.get("user") ?? "";
  if (!TEST_USERS.has(user)) {
    return NextResponse.json({ error: "unknown test user" }, { status: 400 });
  }
  const secret = process.env.JWT_SECRET;
  if (!secret) {
    return NextResponse.json({ error: "server not configured" }, { status: 500 });
  }

  const token = await new SignJWT({})
    .setProtectedHeader({ alg: "HS256" })
    .setSubject(user)
    .setIssuedAt()
    .setExpirationTime("15m")
    .sign(new TextEncoder().encode(secret));

  return NextResponse.json({ token });
}
