# Consumer example

This directory is a separate Go module. It imports the library through
`github.com/DiLRandI/confgen/config` and keeps generated types in `appconfig/`.
The local `replace` directive in `go.mod` points to the parent checkout, so no
published library version is needed to run it.

From this directory:

```sh
go mod download
go generate ./...
DATABASE_URL=postgres://localhost/shop SHOP_SERVER_PORT=10000 go run .
go test ./...
```

Expected output:

```text
server=0.0.0.0:10000 timeout=30s debug=false
```

`config.yaml` sets port 9000. The later environment source overrides it with
10000. The schema supplies the host and timeout defaults. `DATABASE_URL` is
required and secret; the program prints only selected non-secret fields.

- `appconfig/config.schema.yaml` defines the configuration contract.
- `appconfig/doc.go` contains the `go:generate` command.
- `appconfig/config_gen.go` is generated and committed.
- `main.go` loads the file and environment, then uses typed fields.
- `appconfig/example_test.go` contains executable Go documentation examples.

After the library has a release, remove the local replacement with
`go mod edit -dropreplace=github.com/DiLRandI/confgen`, select the release with
`go get github.com/DiLRandI/confgen@<version>`, and run `go mod tidy`.
