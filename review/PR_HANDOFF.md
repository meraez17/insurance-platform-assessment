# Handoff del PR correctivo

## Alcance preparado

Repositorio: `gothinkster/golang-gin-realworld-example-app`  
Base: `626c372d259472148d93303f74aa9b9a1cdcef24`

Cambios:

- Sustituye la clave JWT embebida por `JWT_SECRET` obligatoria de mínimo 32 caracteres.
- Elimina aceptación de tokens en query string.
- Valida el claim `id` antes de convertirlo y evita panic.
- Añade prueba para secretos ausentes/cortos.
- Convierte la prueba de query token en una regresión de seguridad.
- Configura el secreto de prueba en CI.

## Commits recomendados

1. `test(auth): cover unsafe JWT configuration and query tokens`
2. `fix(auth): require external JWT secret and validate claims`
3. `ci(auth): provide isolated secret for test workflow`

## Publicación desde la cuenta de la candidata

1. Crear fork sin abrir PR al repositorio original.
2. Crear rama `fix/auth-hardening` desde el SHA revisado.
3. Aplicar los cambios preparados y conservar los tres commits incrementales.
4. Ejecutar `go test ./...`, `golangci-lint run` y `gosec ./...`.
5. Publicar la rama en el fork.
6. Crear PR **dentro del fork**, por ejemplo desde `fix/auth-hardening` hacia `assessment-base`.
7. Copiar en el README final el enlace del PR y de la ejecución verde.

## Descripción sugerida del PR

### Problema

La aplicación incluye una clave JWT en el código, acepta credenciales por query string y asume que el claim `id` siempre es numérico. En producción esto permite falsificación de tokens, filtración accidental de credenciales y panics ante tokens manipulados.

### Solución

La clave pasa a configuración obligatoria, los tokens solo se aceptan desde `Authorization` y los claims se validan antes de usarse. Se añadieron pruebas de regresión y configuración aislada para CI.

### Riesgo y compatibilidad

El despliegue debe definir `JWT_SECRET`; de lo contrario la emisión de tokens falla de forma segura. Se elimina deliberadamente el soporte de `access_token` en URL. Los clientes que lo usen deben migrar a cabecera antes del despliegue.

### Verificación

- Pruebas unitarias y de middleware.
- Linter.
- Escaneo de seguridad.
- Confirmación de que un token en URL no autentica.

