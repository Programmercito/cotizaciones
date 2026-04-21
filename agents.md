# Agents

Este documento describe la lógica de notificación de Telegram aplicada en `main.go` para el proyecto `cotizaciones`.

## Objetivo

- Guardar umbrales en ejecución cuando no hay valores previos.
- Enviar una nueva notificación sólo cuando el precio actual se sale de los umbrales existentes por más de `±0.30`.
- Si ya existe un mensaje y el precio permanece dentro de los umbrales, editar el mensaje previo en lugar de enviar uno nuevo.

## Lógica actual

1. El bot carga la configuración guardada en la base de datos.
2. Se determina si existe un `messageID` previo.
3. Se evalúa si hay umbrales definidos para USDT y USD Referencial.

### Sin umbrales definidos

- Se guardan los valores actuales de `data.Bid` y `usdRef.Cotizacion` como nuevos umbrales.
- No se envía ni edita ningún mensaje de Telegram en esta ejecución.
- Esto evita notificaciones de arranque y establece las referencias iniciales.

### Con umbrales definidos

- Se calcula la diferencia respecto a los umbrales actuales:
  - `diffUSDT = precioActualUSDT - umbralUSDT`
  - `diffRef = precioActualRef - umbralRef`
- Se considera fuera de umbral cuando `|diff| > 0.30`.

#### Si el precio sale de `±0.30`

- Se envía un mensaje nuevo de spike.
- Se guarda el nuevo `messageID` y los nuevos umbrales basados en los precios actuales.

#### Si el precio permanece dentro de `±0.30`

- Se edita el mensaje existente usando `messageID`.
- Los umbrales no cambian.

## Beneficio

- Reduce ruido de notificaciones para pequeñas variaciones.
- Conserva un solo mensaje cuando el precio no cambia significativamente.
- Actualiza la referencia sólo cuando el precio realmente se aparta del rango aceptable.

## Archivos relevantes

- `main.go`: implementación de la lógica de Telegram, umbrales y mensajes.
- `internal/db/sqlite.go`: almacenamiento y recuperación de `messageID` y umbrales.
- `internal/telegram/bot.go`: envío y edición de mensajes de Telegram.
