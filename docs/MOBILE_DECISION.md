# Memorando de decisión técnica - geolocalización React Native

**Decisión:** no adoptaría `mauron85/react-native-background-geolocation` para una nueva operación productiva. Solo lo mantendría temporalmente en una aplicación existente, encapsulado detrás de una interfaz propia y con un plan de salida.

## Evidencia y riesgo

El repositorio conserva código nativo Android/iOS y licencia Apache 2.0, pero su documentación refleja una base tecnológica antigua: valores por defecto de Android `compileSdkVersion 28`, `targetSdkVersion 28`, `minSdkVersion 16`, referencias de configuración para iOS 11 y configuración manual de módulos nativos. Además, acumula decenas de issues y pull requests abiertos. Para una flota, el riesgo principal no es compilar hoy sino sostener permisos, restricciones de segundo plano, fabricantes agresivos y cambios anuales de Android/iOS.

## Alternativas

| Camino | Ventaja | Costo/riesgo total |
|---|---|---|
| Mantener mauron85 | Sin licencia comercial y migración inmediata baja | Alto costo interno de fork, QA multidispositivo y actualización nativa |
| SDK comercial mantenido | Soporte, actualizaciones y capacidades maduras offline | Licenciamiento y dependencia de proveedor; validar precio contractual |
| Módulos propios con APIs nativas | Control total y ajuste exacto al negocio | Mayor costo inicial, dos especialidades nativas y mantenimiento permanente |

Recomiendo evaluar mediante un spike de dos semanas un SDK comercial mantenido contra un wrapper nativo propio. La decisión debe medir precisión, batería, entrega offline, tasa de pérdida, comportamiento tras reinicio y compatibilidad con los dispositivos reales de la flota. Reversaría la recomendación si el licenciamiento supera el costo de mantener dos componentes nativos o si el proveedor no cumple pruebas de confiabilidad y exportación de datos.

## Diseño de producción

### Offline first

Cada posición lleva `deviceId`, `tripId`, `sequence`, timestamp monotónico y UUID. Se guarda cifrada en SQLite. La sincronización usa lotes de 50-100 puntos, gzip, reintento exponencial con jitter y máximo dos lotes simultáneos. El servidor aplica unicidad por `(deviceId, tripId, sequence)`. Ante backpressure reduce concurrencia; nunca elimina datos no confirmados. La confirmación indica la última secuencia aceptada.

### Batería

No se consulta GPS cada segundo de forma permanente. En movimiento se combinan distancia, actividad y precisión; en detención se usa geofencing o cambios significativos. Android requiere foreground service visible para seguimiento continuo; iOS usa background location, activity type apropiado, pausas automáticas y significant-location changes cuando aplique. El producto permite perfiles por criticidad del viaje.

### Consumo de datos

Supuesto conservador: 120 bytes comprimidos por punto, uno cada 30 segundos durante 8 horas/día y 22 días/mes.

`120 × 120 × 8 × 22 = 2.534.400 bytes`, aproximadamente 2,42 MiB/mes. Aplicando 40% adicional por TLS, confirmaciones y metadatos: 3,4 MiB/mes. Cabe dentro de 20 MB. Un punto cada segundo superaría ampliamente la restricción y no se recomienda salvo eventos críticos y ventanas cortas.

### Permisos y distribución

La solicitud de ubicación en segundo plano debe corresponder a una función central, explicarse al usuario y utilizar solicitudes progresivas. Android y iOS requieren configuración nativa y revisión de tienda. Las actualizaciones OTA se limitan a JavaScript compatible; cambios en permisos, SDK nativo o capacidades de background pasan por tiendas y despliegue escalonado con anillo piloto.

Fuentes de verificación: repositorio público de mauron85 y documentación oficial de ubicación en segundo plano de Android y Apple, consultadas para este assessment.

