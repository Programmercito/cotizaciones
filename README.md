# cotizaciones

Este servicio consulta la API de CriptoYa, guarda la cotización USDT en la
SQLite compartida y genera los JSON consumidos por el frontend y
`cotizacion_social`.

## Responsabilidades

1. Consultar la API y guardar USDT en `/opt/osbo/datausd`.
2. Generar `/opt/osbo/cotiza/social.json`.
3. Exportar todas las cotizaciones a
   `/opt/codes/cotizaciones_ng/docs/data.json` y publicarlas con git.
4. Limpiar registros de más de 90 días.

La generación de imágenes y la comunicación con Telegram pertenecen ahora a
[`cotizacion-telegram`](../cotizacion-telegram), que se ejecuta después de
`cotizacion_social`.

## Desarrollo

```bash
go test ./...
go build -o cotizaciones .
```

Requiere Go 1.27.3 y `TELEGRAM_BOT_TOKEN` ya no es necesario para este servicio.
