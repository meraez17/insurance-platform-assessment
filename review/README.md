# Code review de repositorios públicos

Revisión realizada sobre copias inmutables. Los repositorios son aplicaciones demostrativas; varios atajos son comprensibles para enseñanza, pero no son aceptables en una plataforma financiera, de seguros o telemetría.

| Repositorio | Rama | SHA |
|---|---|---|
| `gothinkster/golang-gin-realworld-example-app` | `main` | `626c372d259472148d93303f74aa9b9a1cdcef24` |
| `gothinkster/aspnetcore-realworld-example-app` | `master` | `a397d1197b22edeffa4d2563fa5f4f7f11d0b254` |
| `reck1ess/next-realworld-example-app` | `master` | `be9ef569bc395975d98c1973fac23ba9277797e7` |

## Go / Gin / GORM

| Sev. | Archivo y línea | Evidencia | Impacto en producción | Recomendación | Esfuerzo |
|---|---|---|---|---|---|
| Crítica | `common/utils.go:40-53` | Clave JWT y contraseña aleatoria constantes en el código. | Cualquiera con el repositorio puede firmar tokens válidos y suplantar usuarios. | Leer el secreto desde un gestor, rotarlo y fallar al iniciar si no existe. | M |
| Alta | `users/middlewares.go:20-24` | Acepta `access_token` en query string. | El token puede quedar en historial, proxies, analítica, referers y logs. | Aceptar exclusivamente cabecera `Authorization`; usar cookies seguras si aplica. | S |
| Alta | `users/middlewares.go:70-72` | Conversión directa `claims["id"].(float64)`. | Un token con claim ausente o tipo inesperado provoca panic y denegación de servicio. | Validar existencia, tipo, rango y sujeto; devolver 401 sin panic. | S |
| Alta | `common/database.go:52-68` | Registra errores de apertura y continúa usando `db`. | El servicio puede arrancar aparentemente sano y fallar durante solicitudes reales. | Retornar error, abortar el arranque y añadir readiness check. | S |
| Media | `hello.go:15-21` | `AutoMigrate` se ejecuta en cada arranque. | Cambios de esquema no auditados, bloqueos y diferencias entre réplicas. | Migraciones versionadas y paso de despliegue separado. | M |
| Media | `common/utils.go:69-72` | Cast directo a `validator.ValidationErrors`. | Errores de binding distintos causan panic y respuesta 500. | Usar `errors.As`, clasificar errores y devolver contrato estable. | S |
| Media | `users/middlewares.go:31-37` | Consulta global a BD en middleware y no valida `db.First` error. | Petición autenticada puede continuar con usuario vacío; añade carga por request. | Validar resultado, cachear identidad de forma segura y desacoplar acceso global. | M |
| Baja | `common/unit_test.go:125-141` | Pruebas afirman longitud del JWT. | Cambios legítimos de claims rompen pruebas sin detectar seguridad o semántica. | Validar firma, expiración y claims, no tamaño de serialización. | S |

## C# / ASP.NET Core / EF Core

| Sev. | Archivo y línea | Evidencia | Impacto en producción | Recomendación | Esfuerzo |
|---|---|---|---|---|---|
| Crítica | `ServicesExtensions.cs:48-53` | Clave simétrica, issuer y audience embebidos. | Firma de tokens comprometida y misma identidad entre ambientes. | Secrets Manager/Key Vault, rotación, validación de configuración y claves por ambiente. | M |
| Crítica | `Infrastructure/Security/PasswordHasher.cs:11-21` | HMAC-SHA512 con clave fija `realworld` y salt concatenado. | Hash rápido, no adaptativo; facilita cracking masivo si se filtra la base. | Argon2id, scrypt o PBKDF2 con parámetros actuales y migración al iniciar sesión. | M |
| Alta | `Program.cs:77,109` | CORS permite cualquier origen, cabecera y método. | Un navegador de origen no confiable puede invocar la API; eleva riesgo de abuso. | Allowlist por ambiente y pruebas de preflight. | S |
| Alta | `Program.cs:120-124` | `EnsureCreated()` durante el arranque. | No hay historial de migración ni control de cambios; riesgo al escalar réplicas. | EF migrations ejecutadas como job único y rollback documentado. | M |
| Media | `DBContextTransactionPipelineBehavior.cs:25-36` | Toda solicitud Mediator abre transacción, incluso lecturas. | Contención y duración innecesaria de transacciones; afecta capacidad. | Aplicar transacción solo a comandos o unidad de trabajo explícita. | M |
| Media | `ErrorHandlingMiddleware.cs:65-75` | Respuesta 500 genérica sin correlation ID ni Problem Details. | Soporte no puede correlacionar error del cliente con trazas y logs. | RFC 7807, trace ID, métricas por clase de excepción y respuesta cancelada. | S |
| Media | `ServicesExtensions.cs:86-94` | Parser manual para esquema `Token`. | Duplica lógica del middleware de autenticación y amplía superficie de error. | Normalizar a Bearer o encapsular un único handler probado. | S |
| Baja | `Program.cs:114-118` | Swagger UI habilitado sin condición de ambiente. | Expone superficie y contratos internos en producción. | Limitar a desarrollo o proteger con autenticación/red privada. | S |

## Next.js / TypeScript / SWR

| Sev. | Archivo y línea | Evidencia | Impacto en producción | Recomendación | Esfuerzo |
|---|---|---|---|---|---|
| Crítica | `pages/article/[pid].tsx:44` | HTML derivado de Markdown se inserta con `dangerouslySetInnerHTML`. | XSS almacenado permite robar tokens y actuar como otra persona. | Sanitizar con allowlist robusta, CSP y pruebas con payloads maliciosos. | M |
| Alta | `components/profile/LoginForm.tsx:33-35` | Usuario y token se guardan en `localStorage`. | Cualquier XSS puede extraer el token persistente. | Sesión en cookie `HttpOnly`, `Secure`, `SameSite`; rotación y expiración corta. | M |
| Alta | `package.json:10-22` | Next 9, React 16, Axios 0.19, TypeScript 3.9 y SWR 0.3. | Dependencias obsoletas, vulnerabilidades conocidas y difícil mantenimiento. | Migración incremental a versiones soportadas, lockfile auditado y Renovate/Dependabot. | L |
| Alta | `lib/api/user.ts:5-100` | `catch` convierte toda falla en `error.response`. | Errores de red no tienen `response`; consumidores reciben `undefined` y fallan lejos de la causa. | Cliente tipado con error de dominio, timeout, cancelación y política uniforme. | M |
| Media | `lib/api/user.ts:7-8` | Lee string de `localStorage` como `any` y consulta `user?.token` sin parsear. | Autenticación actual puede enviar `undefined`; el tipado oculta el defecto. | Parseo seguro, esquema runtime y tipos estrictos sin `any`. | S |
| Media | `lib/utils/fetcher.ts:6-18` | Acceso directo y parsing no protegido de almacenamiento global. | Dato corrupto rompe renderizado; contrato de autenticación duplicado. | Módulo de sesión único, validación de esquema y manejo SSR explícito. | S |
| Media | `components/profile/LoginForm.tsx:23-43` | Inputs y handler sin tipos; errores se imprimen en consola. | Menor testabilidad y posible filtración de detalles en navegador. | Tipar eventos/respuestas, telemetría controlada y mensajes de error seguros. | S |
| Baja | `lib/context/PageContext.tsx:5` | `React.Dispatch<any>`. | Las acciones inválidas solo fallan en ejecución y aumentan regresiones. | Unión discriminada de acciones y reducer exhaustivo. | S |

## Priorización del PR correctivo

Elegiría el repositorio Go para un PR pequeño y demostrable:

1. Extraer el secreto JWT a configuración obligatoria.
2. Eliminar tokens en query string y validar claims sin panic.
3. Añadir pruebas que fallen con claim inválido, método inesperado y secreto ausente.

El alcance combina seguridad, calidad de error y pruebas sin convertir el PR en una reescritura arquitectónica.

