"use client";
import { useEffect, useMemo, useState } from "react";

type OrderStatus = "CREATED"|"QUOTED"|"PAYMENT_PENDING"|"PAID"|"ISSUING"|"ISSUED"|"FAILED"|"REFUND_REQUIRED";
type Order = { id:string; externalRef:string; status:OrderStatus; maskedPlate:string; premiumCents?:number; policyNumber?:string; correlationId:string; createdAt:string; updatedAt:string };

export default function Home() {
  const [orders,setOrders]=useState<Order[]>([]); const [filter,setFilter]=useState<string>(""); const [query,setQuery]=useState("");
  useEffect(()=>{ let active=true; const load=async()=>{const r=await fetch("/api/orders",{cache:"no-store"}); if(r.ok&&active)setOrders(await r.json() as Order[])}; void load(); const id=setInterval(()=>void load(),5000); return()=>{active=false;clearInterval(id)}},[]);
  const visible=useMemo(()=>orders.filter(o=>(!filter||o.status===filter)&&(!query||o.id.toLowerCase().includes(query.toLowerCase())||o.externalRef.toLowerCase().includes(query.toLowerCase()))),[orders,filter,query]);
  return <main><header><div><p className="eyebrow">OPERATIONS</p><h1>Insurance Control Center</h1><p>Seguimiento de pagos, emisiones y excepciones operativas.</p></div><span className="live">● ACTUALIZACIÓN 5 S</span></header>
    <section className="filters"><input aria-label="Buscar" placeholder="Buscar orden o referencia" value={query} onChange={e=>setQuery(e.target.value)}/><select aria-label="Estado" value={filter} onChange={e=>setFilter(e.target.value)}><option value="">Todos los estados</option>{["CREATED","QUOTED","PAYMENT_PENDING","PAID","ISSUING","ISSUED","FAILED","REFUND_REQUIRED"].map(s=><option key={s}>{s}</option>)}</select></section>
    <section className="summary"><article><strong>{orders.length}</strong><span>Total órdenes</span></article><article><strong>{orders.filter(o=>o.status==="ISSUED").length}</strong><span>Emitidas</span></article><article className="warning"><strong>{orders.filter(o=>o.status==="REFUND_REQUIRED").length}</strong><span>Requieren reverso</span></article></section>
    <section className="table"><table><thead><tr><th>Orden</th><th>Estado</th><th>Vehículo</th><th>Prima</th><th>Actualización</th></tr></thead><tbody>{visible.map(o=><tr key={o.id}><td><b>{o.id}</b><small>{o.externalRef}</small></td><td><span className={`status ${o.status.toLowerCase()}`}>{o.status}</span></td><td>{o.maskedPlate}</td><td>{o.premiumCents?new Intl.NumberFormat("es-CO",{style:"currency",currency:"COP",maximumFractionDigits:0}).format(o.premiumCents/100):"—"}</td><td>{new Date(o.updatedAt).toLocaleString("es-CO")}</td></tr>)}</tbody></table>{visible.length===0&&<p className="empty">No hay órdenes que coincidan con el filtro.</p>}</section>
  </main>;
}
