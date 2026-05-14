# Timescale Shadow

El camino principal para NPort debe seguir el modelo `timescaledb_shadow` usado por `modbus-agent`.

## Tabla shadow

Cada ciclo de adquisicion escribe una fila por:

- `plant`
- `device_name`
- `slave_name`
- `ts`

La tabla tiene esta forma logica:

- `plant`
- `ts`
- `device_name`
- `slave_name`
- `series_key`
- `payload` JSONB
- `ingested_at`

## Payload

`payload` debe contener:

- `slave_id`
- `series_key`
- `flags`
- `fields`

Los campos decodificados del equipo van dentro de `payload.fields`. Los campos crudos o auxiliares deben quedar fuera si no sirven para la tabla estructurada.

Los registros crudos secuenciales se normalizan como `value_0`, `value_1`, etc.
Esto mantiene el payload alineado con las columnas estructuradas generadas para
Timescale.

## Configuracion

```yaml
storage:
  outputs:
    - name: local_timescale_shadow
      type: timescaledb_shadow
      enabled: true
      timescaledb_shadow:
        host_env: TIMESCALE_HOST_LOCAL
        port_env: TIMESCALE_PORT_LOCAL
        user_env: TIMESCALE_USER_LOCAL
        password_env: TIMESCALE_PASSWORD_LOCAL
        database_env: TIMESCALE_DB_LOCAL
        schema: edge
        table: stationmeteo_np1_shadow
```

Un mismo puerto NPort puede contener slaves con mapas distintos. Para mantener el
pipeline analogo a `modbus-agent`, se pueden declarar varias salidas
`timescaledb_shadow` y filtrar por slave o por `device_type`:

```yaml
storage:
  outputs:
    - name: local_timescale_kipp_zonen_shadow
      type: timescaledb_shadow
      enabled: true
      timescaledb_shadow:
        host_env: TIMESCALE_HOST_LOCAL
        port_env: TIMESCALE_PORT_LOCAL
        user_env: TIMESCALE_USER_LOCAL
        password_env: TIMESCALE_PASSWORD_LOCAL
        database_env: TIMESCALE_DB_LOCAL
        schema: edge
        table: nport_kipp_zonen_shadow
        device_types:
          - kipp_zonen

    - name: local_timescale_dustiq_shadow
      type: timescaledb_shadow
      enabled: true
      timescaledb_shadow:
        host_env: TIMESCALE_HOST_LOCAL
        port_env: TIMESCALE_PORT_LOCAL
        user_env: TIMESCALE_USER_LOCAL
        password_env: TIMESCALE_PASSWORD_LOCAL
        database_env: TIMESCALE_DB_LOCAL
        schema: edge
        table: nport_dustiq_shadow
        device_types:
          - dustiq
```

Filtros disponibles:

- `device_types`
- `slave_names`
- `slave_ids`

Si una salida no declara filtros, recibe todas las muestras, que es el
comportamiento historico.

## Flujo Ansible

El rol `deploy_nport_acquisition` en `ansible-infra` renderiza las variables `TIMESCALE_*` desde:

- `inventories/prod_inventory/vaults/timescale.yml`
- `inventories/prod_inventory/vaults/hosts/{{ inventory_hostname }}.yml`

Luego el flujo existente de Timescale puede publicar, subscribir y transformar las tablas `*_shadow` igual que con `modbus-agent`.
