import { NextRequest, NextResponse } from "next/server";

export async function POST(request: NextRequest, context: { params: Promise<{ id: string; action: string }> }) {
  const { id, action } = await context.params;
  if (!new Set(["retry-issuance", "mark-refund-required"]).has(action)) return NextResponse.json({ error: "invalid action" }, { status: 404 });
  const base = process.env.ORCHESTRATOR_INTERNAL_URL ?? "http://localhost:8080";
  const response = await fetch(`${base}/orders/${encodeURIComponent(id)}/${action}`, { method: "POST", headers: { "X-Actor-ID": request.headers.get("X-Actor-ID") ?? "operations-console" } });
  return new NextResponse(await response.text(), { status: response.status, headers: { "content-type": "application/json" } });
}
