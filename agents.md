# Agents

Referencia compacta de la arquitectura de cotizaciones.

## Proyectos

- `cotizaciones` consulta la API, escribe SQLite y publica JSON.
- `cotizacion_social` lee `social.json` y genera `usdt.jpg` y `resto.jpg`.
- `cotizacion-telegram` lee la misma DB, consume esas imágenes y gestiona
  Telegram, edición, spikes, umbrales e IDs.

## Build & run

- Go 1.27.3.
- `go test ./...`
- `go build -o cotizaciones .`

## Rutas de producción

- SQLite: `/opt/osbo/datausd`
- JSON frontend: `/opt/codes/cotizaciones_ng/docs/data.json`
- JSON social: `/opt/osbo/cotiza/social.json`
- Imagen USDT: `/opt/osbo/cotiza/usdt.jpg`
- Imagen resto: `/opt/osbo/cotiza/resto.jpg`
- Lock de Telegram: `/opt/osbo/cotizaciones.lock`

## Secuencia de CronJob

El manifiesto en `bot-dockers/cotizaciones/kub/services.yaml` ejecuta:

1. `cotizaciones`
2. `cotizaciones_social`
3. `cotizacion-telegram`

Las dos primeras etapas son `initContainers`; Telegram es el contenedor final
del Job y solo arranca cuando las imágenes ya fueron generadas.
