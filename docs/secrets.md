# Secretos

El agente no debe recibir tokens directos en los YAML versionados.

## Contrato

Las configuraciones de NPort deben referenciar variables de entorno para credenciales, por ejemplo:

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
        table: petorca_stationmeteo_np1_shadow
        device_types:
          - kipp_zonen
```

## Fuente de verdad

Los secretos viven en Ansible Vault dentro de `ansible-infra`:

- `inventories/prod_inventory/vaults/influx.yml`
- `inventories/prod_inventory/vaults/timescale.yml`
- `inventories/prod_inventory/vaults/hosts/{{ inventory_hostname }}.yml`

El rol `deploy_nport_acquisition` renderiza un archivo `.env` con variables `INFLUX_*` y `TIMESCALE_*`.
