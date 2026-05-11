# Roadmap inicial

## Paso 1: base del repo

- Crear estructura del agente y despliegue.
- Definir convenciones para configs por sitio.
- Evitar versionar tokens y credenciales.

## Paso 2: migracion Petorca

- Importar la app Go actual.
- Separar configuracion comun, sitio, NPorts y destinos de almacenamiento.
- Reemplazar tokens directos por variables o archivos secretos administrados fuera del repo.

## Paso 3: Ansible

- Mantener el rol/playbook en `/home/angel/atamostec/ansible-infra`.
- Instalar binario del agente.
- Renderizar configuracion por host/planta.
- Crear servicio systemd/timer.
- Configurar directorio textfile collector de node_exporter.

## Paso 4: endurecimiento

- Tests de parsing Modbus/CRC.
- Validacion de YAML.
- Modo dry-run para validar config sin conectar a NPorts.
- Logs estructurados.
