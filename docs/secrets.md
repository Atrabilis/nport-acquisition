# Secretos

El agente no debe recibir tokens directos en los YAML versionados.

## Contrato

Las configuraciones de NPort deben referenciar variables de entorno para credenciales, por ejemplo:

```yaml
storage:
  local:
    - name: local
      db-type: influxdb2
      db-url-env: INFLUX_HOST_LOCAL
      db-token-env: INFLUX_TOKEN_LOCAL
      db-org-env: INFLUX_ORG_LOCAL
      db-bucket-env: INFLUX_BUCKET_LOCAL
      db-measurement: petorca-stationmeteo-np1
  remotes:
    - name: remote
      db-type: influxdb2
      db-url-env: INFLUX_HOST_REMOTE
      db-token-env: INFLUX_TOKEN_REMOTE
      db-org-env: INFLUX_ORG_REMOTE
      db-bucket-env: INFLUX_BUCKET_REMOTE
      db-measurement: petorca-stationmeteo-np1
```

## Fuente de verdad

Los secretos viven en Ansible Vault dentro de `ansible-infra`:

- `inventories/prod_inventory/vaults/influx.yml`
- `inventories/prod_inventory/vaults/hosts/{{ inventory_hostname }}.yml`

El rol `deploy_nport_acquisition` renderiza un archivo `.env` con variables `INFLUX_*`.

