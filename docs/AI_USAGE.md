# Registro de uso de IA

| Momento | Objetivo | Propuesta de IA | Decisión humana | Corrección o descarte |
|---|---|---|---|---|
| Diseño inicial | Reducir el alcance a un MVP defendible | Monorepo con Go, .NET, Next.js, simuladores y Docker Compose | Aceptada | Se excluyó el prototipo móvil opcional |
| Resiliencia | Resolver timeouts ambiguos del legado | Consultar por `externalRef` antes de reintentar | Aceptada | Se prohibió reemitir basándose solo en timeout |
| Compensación | Tratar pago aprobado con emisión fallida | Estado `REFUND_REQUIRED` visible a operaciones | Aceptada | Se descartó el reembolso automático por riesgo de póliza ya emitida |
| Frontend | Crear consola con actualización en tiempo real | WebSockets | Ajustada | Se eligió polling de 5 segundos por simplicidad operacional y escala del assessment |
| Móvil | Adoptar librería open source asignada | Mantenerla por costo de licencia cero | Descartada | La antigüedad nativa traslada el costo al equipo y eleva el riesgo operativo |
| AWS | Usar EKS por estandarización de microservicios | EKS | Descartada | ECS Fargate reduce carga operativa y cubre la escala planteada |

## Regla de trabajo

Toda propuesta generada con IA se revisa contra el enunciado, se prueba y se puede explicar. No se incorpora código cuya intención, riesgo y comportamiento ante fallos no puedan defenderse en la sustentación.
