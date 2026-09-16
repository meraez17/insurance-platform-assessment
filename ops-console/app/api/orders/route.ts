import { NextResponse } from "next/server";
export async function GET(){const base=process.env.ORCHESTRATOR_INTERNAL_URL??"http://localhost:8080"; const response=await fetch(`${base}/orders`,{cache:"no-store"}); return new NextResponse(await response.text(),{status:response.status,headers:{"content-type":"application/json"}})}
