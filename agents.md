# Agents

Este documento describe la lógica completa de la app `cotizaciones` y su comportamiento real en `main.go`.

## Visión general

La app realiza estas tareas principales en cada ejecución:

1. Consulta la API de CriptoYa para obtener la cotización de USDT y otros valores.
2. Abre la base de datos SQLite local.
3. Guarda la cotización USDT en la tabla `cotizaciones`.
4. Genera un resumen con los últimos valores de USDT, USD Oficial, USD Referencial, Euro, Oro, Plata y UFV.
5. Intenta crear una imagen de la cotización para Telegram.
6. Lee la configuración de `config` y decide si debe enviar o editar un mensaje de Telegram.
7. Exporta todas las cotizaciones a JSON para el frontend.
8. Actualiza el repo del frontend con `git commit` / `git push`.
9. Elimina registros de cotizaciones mayores a 30 días.

## Umbrales separados

La lógica usa dos umbrales independientes:

- `umbral`: referencia para el precio de USDT.
- `umbral_referencial`: referencia para el USD Referencial.

Ambos umbrales se manejan por separado y tienen reglas distintas de inicialización y actualización.

## Lógica de inicialización de umbrales

- Si `cfg.Umbral` es `null`, el umbral USDT no está inicializado.
- Si `cfg.UmbralReferencial` es `null`, el umbral USD Referencial no está inicializado.
- Si alguno de los dos está `null`, la app:
  - guarda el valor actual de `data.Bid` como `umbral`,
  - guarda el valor actual de `usdRef.Cotizacion` como `umbral_referencial`,
  - no envía ni edita ningún mensaje de Telegram en esa ejecución,
  - conserva el `messageID` existente si ya lo tiene.

Esta ejecución inicial solo establece las referencias para la siguiente comparación.

## Comparación de variaciones

Cuando ambos umbrales ya existen, la app calcula:

- `diffUSDT = data.Bid - currentUmbralUSDT`
- `diffRef = usdRef.Cotizacion - currentUmbralRef`

Y luego evalúa:

- `outsideUSDT := math.Abs(diffUSDT) > 0.30`
- `outsideRef := math.Abs(diffRef) > 0.30`
- `isOutside := outsideUSDT || outsideRef`

### Qué significa esto

- Si `outsideUSDT` o `outsideRef` es `true`, entonces hay un cambio significativo en alguno de los dos precios.
- Si ambos son `false`, el precio está dentro del rango permitido y no debe reiniciarse el umbral.

## Qué hace la app en cada caso

### 1. No hay `messageID` válido

- Envía un mensaje inicial de resumen diario.
- Guarda solo el `messageID` en la base de datos.
- No actualiza los umbrales, porque esos ya se gestionan en otro paso.

### 2. Spike detectado (`isOutside == true`)

- Envía un nuevo mensaje de spike.
- Actualiza el `messageID` con el mensaje recién creado.
- Actualiza ambos umbrales (`umbral` y `umbral_referencial`) con los valores actuales.
- A partir de ese nuevo spike, esos nuevos umbrales se usan como referencia.

### 3. Dentro de rango (`isOutside == false`)

- Intenta editar el mensaje existente en Telegram.
- Si la edición falla, envía un nuevo mensaje y actualiza solo el `messageID`.
- Bajo ninguna circunstancia se actualizan los umbrales en este camino.

## Spike: comportamiento correcto

El spike ocurre cuando alguno de los dos precios supera `±0.30` respecto a su umbral.

- Para USDT:
  - `diffUSDT = data.Bid - currentUmbralUSDT`
- Para USD Referencial:
  - `diffRef = usdRef.Cotizacion - currentUmbralRef`

El spike muestra el mayor de los dos cambios para el mensaje visual y guarda el nuevo estado.

### Resultado del spike

- Se envía un alerta de subida o bajada.
- Se actualizan los umbrales al valor actual.
- Se guarda el nuevo `messageID`.

## Guardado en la base de datos

### `internal/db/sqlite.go`

- `GetConfig()` lee la fila única de configuración.
- `UpdateConfig(currentDate, messageID, umbralUSDT, umbralRef)` actualiza `messageID` y ambos umbrales.
- `UpdateConfigMessageID(currentDate, messageID)` actualiza solo `messageID`, preservando los umbrales.

## Mensajes de Telegram

### Mensaje diario

- `FormatDailyMessage(summary)` genera el resumen normal.
- Incluye USDT, USD Oficial, USD Referencial, Euro, Oro, Plata y UFV.
- Se usa cuando el precio no sale de rango o cuando no hay spike.

### Mensaje de spike

- `FormatSpikeMessage(summary, currentUmbralUSDT, diff, diff > 0)` genera el alerta.
- La alerta informa la variación frente al umbral y el tipo de movimiento.
- Se envía solo cuando hay un cambio significativo en USDT o USD Referencial.

## Flujo completo de `main.go`

1. `api.FetchCotizacion()` obtiene los datos.
2. `db.New()` abre la base de datos.
3. `database.InsertCotizacion(data.Bid, data.TotalAsk)` guarda la cotización.
4. `database.GetLatestSummary()` construye el resumen para Telegram.
5. `telegram.GeneratePriceImage(summary)` intenta crear la imagen.
6. `database.GetConfig()` lee la configuración actual.
7. Se calcula si los umbrales están inicializados.
8. Si no lo están, se guardan y se retorna sin notificar.
9. Si ya están inicializados, se evalúa `isOutside`.
10. Si hay spike, se envía spike + actualiza umbrales.
11. Si no hay spike, se edita el mensaje existente + guarda solo `messageID` si es necesario.
12. `database.ExportCotizacionesToJSON(jsonOutputPath)` escribe el JSON.
13. `git.ForcePull(ngRepoPath)` y `git.CommitAndPush(ngRepoPath, commitMsg)` actualizan el frontend.
14. `database.DeleteOlderThan(30 * 24 * time.Hour)` limpia datos viejos.

## Componentes relevantes

- `main.go`
- `internal/db/sqlite.go`
- `internal/api/criptoya.go`
- `internal/telegram/bot.go`
- `internal/telegram/image.go`
- `internal/git/git.go`

## Correcciones clave

- Los umbrales son independientes y se manejan por separado.
- Si cualquiera está `null`, la app los inicializa y no notifica.
- Si el precio queda dentro de `±0.30`, no se tocan los umbrales.
- Solo se actualizan los umbrales cuando hay spike.
- La edición de mensaje no recrea la referencia de umbral.

## Resumen rápido

- `null` → inicializar umbrales y no notificar.
- `isOutside == false` → editar mensaje, no cambiar umbrales.
- `isOutside == true` → enviar spike, actualizar umbrales y messageID.

