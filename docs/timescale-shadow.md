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

## Flujo Ansible

El rol `deploy_nport_acquisition` en `ansible-infra` renderiza las variables `TIMESCALE_*` desde:

- `inventories/prod_inventory/vaults/timescale.yml`
- `inventories/prod_inventory/vaults/hosts/{{ inventory_hostname }}.yml`

Luego el flujo existente de Timescale puede publicar, subscribir y transformar las tablas `*_shadow` igual que con `modbus-agent`.

