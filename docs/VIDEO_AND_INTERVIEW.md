# Guion de sustentación (10-15 minutos)

## 0:00-1:00 - Problema y prioridad

El riesgo central no es crear una orden: es mantener consistencia cuando pago y póliza viven en terceros que fallan de manera independiente. La decisión rectora es evitar cobros o emisiones duplicadas.

## 1:00-4:00 - Flujo end-to-end

Demostrar creación con `Idempotency-Key`, cotización, `PAYMENT_PENDING`, webhook firmado, emisión y consola. Repetir la creación con la misma clave y mostrar que retorna la misma orden. Reutilizar la clave con otro cuerpo y mostrar conflicto.

## 4:00-6:30 - Fallos y recuperación

Activar la caída del emisor. Explicar outbox, reintentos, jitter, DLQ y circuit breaker. Ejecutar el escenario "timeout pero sí emitió" y mostrar que reconciliación consulta `externalRef` antes de reintentar. Explicar por qué `REFUND_REQUIRED` no ejecuta un reembolso automático.

## 6:30-8:00 - Capa anticorrupción

Mostrar contrato interno limpio en C#, mapeo del contrato legado y conversión de errores de negocio dentro de HTTP 200.

## 8:00-10:30 - Code review y liderazgo

Presentar solo los hallazgos de mayor impacto: secretos JWT, hashing, XSS y token en localStorage. Explicar la priorización del PR y leer el mensaje al senior resaltando tono, autonomía y acción.

## 10:30-12:00 - Móvil y AWS

Justificar por qué no se adopta la librería asignada para una nueva implementación. Mostrar ECS Fargate, RDS Multi-AZ y Amazon MQ; conectar decisiones con escala y costo operativo.

## 12:00-13:30 - IA, renuncias y cierre

Mostrar el registro de IA, una recomendación aceptada, una corregida y una descartada. Cerrar con las renuncias deliberadas: sin prototipo móvil opcional, sin Terraform productivo y sin features accesorias.

## Preguntas previsibles de entrevista

1. **¿Por qué outbox si RabbitMQ es durable?** Porque la durabilidad del broker no resuelve la ventana entre commit de BD y publicación.
2. **¿Por qué entrega al menos una vez?** Exactamente una vez extremo a extremo no es realista; se logra efecto único mediante idempotencia.
3. **¿Qué haces ante timeout del emisor?** Resultado desconocido: consultar por referencia externa antes de reemitir.
4. **¿Por qué no reembolso automático?** La póliza puede existir aunque la respuesta se haya perdido; automatizar puede regalar cobertura y devolver dinero.
5. **¿Por qué ECS y no EKS?** Menor carga operativa para la escala planteada; revisable si ya existe plataforma Kubernetes madura.
6. **¿Qué recortaste?** Prototipo móvil, IaC completo y funcionalidades visuales; nunca consistencia, seguridad ni pruebas críticas.
7. **¿Cómo manejas un webhook adelantado?** Inbox persistente, estado `DEFERRED` y replay cuando la orden sea procesable.
8. **¿Cómo evaluarías a quien usó IA?** Cambios en vivo, explicación de supuestos, depuración y defensa de decisiones descartadas.

