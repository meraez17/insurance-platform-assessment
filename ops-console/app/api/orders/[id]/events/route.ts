import { NextRequest, NextResponse } from "next/server";

export async function GET(_request: NextRequest, context: { params: Promise<{ id: string }> }) {
  const { id } = await context.params;
  const base = process.env.ORCHESTRATOR_INTERNAL_URL ?? "http://localhost:8080";
  const response = await fetch(`${base}/orders/${encodeURIComponent(id)}/events`, { cache: "no-store" });
  return new NextResponse(await response.text(), { status: response.status, headers: { "content-type": "application/json" } });
}
