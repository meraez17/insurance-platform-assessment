# Liderazgo, contratación y estándares

## Mensaje al desarrollador senior

Revisé el cambio priorizando riesgo de producción. Hay tres puntos que debemos corregir antes de aprobar: el flujo de autenticación permite estados no controlados, los errores del tercero pierden contexto y las pruebas validan el camino feliz sin cubrir duplicados ni timeouts. No son observaciones de estilo; pueden exponer datos o dejar operaciones en un estado inconsistente.

Propongo dividir el ajuste así: primero asegurar autenticación y manejo de secretos; segundo normalizar errores y conservar correlation ID; tercero agregar pruebas que reproduzcan el fallo antes de corregirlo. En el PR dejé ejemplos concretos y separé recomendaciones no bloqueantes.

Quiero que tú plantees la solución final y expliques el trade-off elegido. Si detectas que alguna recomendación introduce complejidad innecesaria para este repositorio, documéntalo y lo revisamos. El objetivo no es solo cerrar este PR: necesitamos que el patrón quede claro y reutilizable para el equipo.

## Filtro para senior fullstack

Ejercicio de cuatro horas: implementar una orden idempotente que persista estado, publique un evento y muestre su evolución en React. Se entrega un tercero que responde lento, duplica callbacks y devuelve un error de negocio con HTTP 200. No se evalúa cantidad de pantallas.

Avanza quien delimita alcance, modela estados, prueba fallos, protege secretos, explica consistencia y deja commits comprensibles. Se descarta aunque funcione si oculta errores, confunde timeout con fallo, usa `any`, registra datos sensibles, no puede explicar el código o introduce infraestructura desproporcionada.

Si usó IA, se le pide cambiar una regla en vivo, identificar una suposición incorrecta, explicar una decisión descartada y depurar una prueba inesperada. No vale evaluar memorización de sintaxis ni algoritmos ajenos al trabajo; esos riesgos se cubren con conversación técnica, referencias y periodo de prueba.

## Definition of Done

- Criterios de aceptación cubiertos y alcance explícito.
- Pruebas unitarias e integración para caminos críticos y fallos.
- Linter, análisis estático y pipeline verdes.
- Seguridad: secretos, autorización, datos sensibles y dependencias revisados.
- Observabilidad y correlation ID incluidos.
- Migraciones reversibles y compatibilidad evaluada.
- Documentación, runbook y decisiones actualizados.
- Code review aprobado y evidencia funcional adjunta.
- Estrategia de despliegue y reversa definida.

## Checklist de PR

1. ¿El cambio resuelve un criterio de aceptación verificable?
2. ¿Se preserva idempotencia, consistencia y compatibilidad?
3. ¿Errores y timeouts tienen tratamiento explícito?
4. ¿Autenticación y autorización se aplican en servidor?
5. ¿No existen secretos ni datos sensibles en código o logs?
6. ¿Las pruebas fallarían si se elimina el arreglo?
7. ¿Consultas, llamadas externas y renderizado tienen límites?
8. ¿Métricas y logs permiten diagnosticar producción?
9. ¿La migración y el rollback son seguros?
10. ¿La complejidad añadida es proporcional al problema?

