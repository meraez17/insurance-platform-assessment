# Plan de pruebas y evidencia

| Caso | Acción | Resultado esperado |
|---|---|---|
| Creación normal | POST `/orders` con clave única | 202 y estado `CREATED` |
| Replay idempotente | Repetir clave y cuerpo | 200 y mismo `orderId` |
| Conflicto de clave | Misma clave, placa diferente | 409 |
| Firma inválida | Webhook con HMAC incorrecto | 401, sin transición |
| Webhook duplicado | Enviar dos veces `eventId` | Segundo retorna 204 sin efecto adicional |
| Evento adelantado | APPROVED antes de `PAYMENT_PENDING` | Inbox `DEFERRED`; luego replay |
| Timeout con emisión | Legado guarda y responde 504 | Consulta por `externalRef`; una sola póliza |
| Caída del emisor | Activar `/chaos/issuer-outage` | Circuit abierto, orden observable y reintento diferido |
| Agotamiento | Forzar ocho fallos | Mensaje en DLQ; orden no se pierde |
| Fallo definitivo | Pago aprobado y emisión no recuperable | `REFUND_REQUIRED`, sin devolución automática |
| Datos sensibles | Revisar logs | Placa y documento no aparecen completos |
| Operación manual | Reintentar con `X-Actor-ID` | Evento auditado con actor |

## Evidencias para la entrega

- Salida de pruebas unitarias Go y .NET.
- Compilación TypeScript estricta.
- Capturas o grabación de estados en la consola.
- Consulta SQL de outbox/inbox y eventos.
- Logs con correlation ID durante un fallo.
- Ejecución verde del workflow en GitHub.

