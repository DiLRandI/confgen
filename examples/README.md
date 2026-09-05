# Application example

This is a separate consumer module with an ordinary dependency on confgen.
Its schema defines defaults and a required database URL. The program loads
`config.yaml`, then applies environment overrides.

From this directory:

```sh
go mod download
go generate ./...
DATABASE_URL=postgres://localhost/shop SHOP_SERVER_PORT=10000 go run .
go test ./...
```

Output: `server=0.0.0.0:10000 timeout=30s debug=false`.

The schema defaults port to 8080, the file changes it to 9000, and the environment
changes it to 10000. Copy `appconfig/` into your application and update its import
in `main.go`. The [README quick start](../README.md#quick-start) is a smaller example.
