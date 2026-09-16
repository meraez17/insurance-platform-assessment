# Estrategia de resiliencia

## Garantías

- La API acepta una orden una sola vez por combinación `Idempotency-Key` + hash de solicitud.
- Reutilizar la misma clave con otro cuerpo devuelve conflicto; no retorna silenciosamente la orden anterior.
- Cada transición y su evento de salida se confirman en una única transacción PostgreSQL.
- La entrega al broker es al menos una vez; consumidores e inbox neutralizan duplicados.
- Los eventos de pago tempranos se guardan como `DEFERRED` y se reproducen cuando la orden alcanza `PAYMENT_PENDING`.
- Los workers reclaman mensajes con `FOR UPDATE SKIP LOCKED`, permitiendo escalar horizontalmente.

## Política de reintentos

Máximo ocho intentos, backoff exponencial con jitter y DLQ al agotar el presupuesto. Los errores de negocio no se reintentan. Los fallos temporales sí. Un resultado desconocido por timeout entra en reconciliación, no en reemisión directa.

## Circuit breaker

Se abre cuando cinco solicitudes consecutivas fallan o superan el timeout. Permanece abierto 30 segundos y luego permite una solicitud de prueba. Mientras está abierto, la orden permanece en `ISSUING`, el mensaje se reprograma y la situación es observable.

## Reconciliación

Cada minuto se consultan órdenes en `ISSUING` sin cambios durante más de cinco minutos:

1. Consultar `GET /policies/by-reference/{externalRef}`.
2. Si existe, guardar el número y avanzar a `ISSUED`.
3. Si no existe y quedan intentos, reprogramar la emisión.
4. Si el fallo es definitivo después del pago, avanzar a `REFUND_REQUIRED`.
5. Nunca ejecutar un reembolso automático: el dinero y la póliza están en sistemas con consistencia eventual y el timeout puede ocultar una emisión exitosa.

