
## Testing

```bash
go vet ./...
go test ./...
```

`cmd/nport-agent/e2e_test.go` builds the agent and runs it in `store` mode against a fake NPort (a TCP server replaying Modbus RTU bus traffic: noise, master requests, slave responses, and two responses with no silence between them) and a real PostgreSQL shadow table. It checks one row per expected slave with all 43 Kipp & Zonen registers, that the run stops as soon as every expected slave was captured, and the discovery report. A second test documents current behavior: when `store_timeout_seconds` runs out before every slave in `detected_slaves` was seen, nothing is written.

They are skipped unless `NPORT_E2E_PG` points to a PostgreSQL server:

```bash
docker run -d --rm --name np-e2e-pg -e POSTGRES_PASSWORD=e2e -p 127.0.0.1:55434:5432 postgres:17.9
NPORT_E2E_PG=postgres://postgres:e2e@127.0.0.1:55434/postgres go test ./cmd/nport-agent
docker stop np-e2e-pg
```

CI runs them with a PostgreSQL service and publishes the merged coverage (tests and binary) in the job summary.

## License

MIT. See [LICENSE](LICENSE).
