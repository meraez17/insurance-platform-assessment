import crypto from "node:crypto";
import express, { Request, Response } from "express";

const app = express();
app.use(express.json());

const issued = new Map<string, string>();
let issuerDownUntil = 0;

app.get("/health", (_req, res) => res.json({ status: "ok" }));

app.post("/quote", async (req: Request, res: Response) => {
  await delay(random(30, 250));
  if (Math.random() < 0.05) return res.status(422).json({ code: "INVALID_FORMAT" });
  if (!req.body?.plate) return res.status(400).json({ code: "PLATE_REQUIRED" });
  return res.json({ quoteId: crypto.randomUUID(), premiumCents: 125_000, validityDays: 30 });
});

app.post("/payments", (req: Request, res: Response) => {
  if (!req.body?.orderId) return res.status(400).json({ code: "ORDER_REQUIRED" });
  const event = { eventId: crypto.randomUUID(), orderId: req.body.orderId, status: "APPROVED" };
  setTimeout(() => void sendWebhook(event), random(100, 800));
  return res.status(202).json({ paymentId: crypto.randomUUID(), status: "PENDING" });
});

app.post("/legacy/issue", async (req: Request, res: Response) => {
  const ref = String(req.body?.external_reference ?? "");
  if (!ref) return res.status(200).json({ ok: false, business_error: "REFERENCE_REQUIRED" });
  if (Date.now() < issuerDownUntil) return res.status(503).json({ message: "issuer unavailable" });
  await delay(random(100, 1_200));
  const existing = issued.get(ref);
  if (existing) return res.json({ ok: true, policy_no: existing, duplicate: true });
  const policy = `POL-${crypto.randomInt(100000, 999999)}`;
  issued.set(ref, policy);
  if (Math.random() < 0.2) return res.status(504).json({ message: "timeout after processing" });
  if (Math.random() < 0.1) return res.status(200).json({ ok: false, business_error: "VEHICLE_NOT_ELIGIBLE" });
  return res.json({ ok: true, policy_no: policy, redundant_status: "SUCCESS" });
});

app.get("/legacy/policies/:externalRef", (req, res) => {
  const policy = issued.get(req.params.externalRef);
  return policy ? res.json({ found: true, policy_no: policy }) : res.status(404).json({ found: false });
});

app.post("/chaos/issuer-outage", (req, res) => {
  const seconds = Math.min(Number(req.body?.seconds ?? 30), 300);
  issuerDownUntil = Date.now() + seconds * 1000;
  res.json({ issuerDownUntil: new Date(issuerDownUntil).toISOString() });
});

async function sendWebhook(event: object): Promise<void> {
  const body = JSON.stringify(event);
  const secret = process.env.WEBHOOK_HMAC_SECRET ?? "local-development-secret";
  const signature = crypto.createHmac("sha256", secret).update(body).digest("hex");
  const url = process.env.ORCHESTRATOR_WEBHOOK_URL ?? "http://localhost:8080/webhooks/payments";
  await fetch(url, { method: "POST", headers: { "content-type": "application/json", "x-signature": signature }, body });
  if (Math.random() < 0.1) {
    await fetch(url, { method: "POST", headers: { "content-type": "application/json", "x-signature": signature }, body });
  }
}

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
const random = (min: number, max: number) => crypto.randomInt(min, max + 1);
app.listen(8082, () => console.log("simulators listening on :8082"));

