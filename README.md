# NPort Acquisition

Repositorio base para generalizar la adquisicion de datos desde NPort/Modbus y desplegarla con Ansible.

## Objetivo

Unificar los scripts hechos por planta en un agente configurable, con:

- configuracion por sitio/planta sin secretos versionados;
- despliegue reproducible por Ansible;
- salida hacia InfluxDB y node_exporter textfile;
- soporte incremental para distintos tipos de equipos conectados a NPort.

## Estructura

- `cmd/nport-agent/`: punto de entrada futuro del agente Go.
- `internal/`: paquetes internos del agente.
- `configs/sites/`: plantillas y ejemplos de configuracion por planta.
- `docs/`: notas de migracion y decisiones de diseno.

## Proximo paso sugerido

Tomar una configuracion existente, por ejemplo Petorca, y convertirla en una plantilla sin tokens. Despues se migra la logica Go actual al layout nuevo manteniendo compatibilidad con el YAML mientras ordenamos el modelo de configuracion.

El despliegue Ansible vive en `/home/angel/atamostec/ansible-infra`.
El contrato de secretos esta descrito en `docs/secrets.md`.
El contrato de Timescale shadow esta descrito en `docs/timescale-shadow.md`.
