
## Testing

```bash
go vet ./...
go test ./...
```

`cmd/nport-agent/e2e_test.go` builds the agent and runs it in `store` mode against a fake NPort (a TCP server replaying Modbus RTU bus traffic: noise, master requests, slave responses, and two responses with no silence between them) and a real PostgreSQL shadow table. It checks one row per expected slave with all 43 Kipp & Zonen registers, that the run stops as soon as every expected slave was captured, and the discovery report. A second test covers a slave that never answers: when `store_timeout_seconds` runs out before every slave in `detected_slaves` was seen, the captured slaves are still written and the missing ones are logged (`[store] WARNING: run ended before every expected slave was captured; ... missing: <port>/<slave_id>`). Before 2026-10-27 nothing was written in that case, so one disconnected sensor lost the data of every other sensor on the same port.

They are skipped unless `NPORT_E2E_PG` points to a PostgreSQL server:

```bash
docker run -d --rm --name np-e2e-pg -e POSTGRES_PASSWORD=e2e -p 127.0.0.1:55434:5432 postgres:17.9
NPORT_E2E_PG=postgres://postgres:e2e@127.0.0.1:55434/postgres go test ./cmd/nport-agent
docker stop np-e2e-pg
```

CI runs them with a PostgreSQL service and publishes the merged coverage (tests and binary) in the job summary.

## License

MIT. See [LICENSE](LICENSE).
