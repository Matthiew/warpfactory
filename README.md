# Catalog service

A Go service with the same fake books and cars over HTTP JSON and protobuf gRPC. The `catalog_label` in every response comes from a JSON file and changes after the process receives `SIGHUP`. HTTP and gRPC listen on separate ports. Listen addresses are startup flags and do not change on reload.

## Run locally

Requires Go 1.24 or newer.

```sh
go run ./cmd/catalog -config config.json -http-addr :8080 -grpc-addr :9090
```

`GET http://localhost:8080/books` returns `{"catalog_label":"demo","books":[...]}`. `GET http://localhost:8080/cars` returns `{"catalog_label":"demo","cars":[...]}`. gRPC exposes `catalog.v1.Catalog/ListBooks` and `catalog.v1.Catalog/ListCars` on port 9090; the checked-in proto is `proto/catalog/v1/catalog.proto`. For example, with `grpcurl` installed:

```sh
grpcurl -plaintext -import-path proto -proto catalog/v1/catalog.proto localhost:9090 catalog.v1.Catalog/ListBooks
```

To test reload, build and start the binary, then atomically replace its config file and signal its PID:

```sh
go build -o catalog ./cmd/catalog
config_copy=$(mktemp)
cp config.json "$config_copy"
./catalog -config "$config_copy" &
pid=$!
until curl -fsS http://localhost:8080/books >/dev/null; do sleep 0.1; done
printf '{"catalog_label":"new catalog"}\n' > "$config_copy.next"
mv "$config_copy.next" "$config_copy"
kill -HUP "$pid"
until curl -fsS http://localhost:8080/books | grep -q '"catalog_label":"new catalog"'; do sleep 0.1; done
curl http://localhost:8080/books
kill "$pid"
```

The process logs a rejected reload and keeps the last valid label if the file is missing, malformed, contains unknown fields, or has a blank `catalog_label`. Use an atomic file replacement in production so the process never reads a partial write.

## Regenerate protobuf code

The generated Go bindings under `gen/catalog/v1` are committed. With `protoc` 33.0 and Go on `PATH`, regenerate them using:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
protoc -I . --go_out=. --go_opt=module=github.com/Matthiew/warpfactory --go-grpc_out=. --go-grpc_opt=module=github.com/Matthiew/warpfactory proto/catalog/v1/catalog.proto
```

Ensure `$(go env GOPATH)/bin` is on `PATH` for `protoc` to find the plugins. Run `go test -race ./...`, `go vet ./...`, and `go build ./...` to verify.

## Docker and Nomad

`docker build -t catalog:local .` builds an image that runs the service binary directly as PID 1. Publish that image and set its tag in `nomad/catalog.nomad.hcl` before scheduling. The example job renders `local/config.json` (visible as `/local/config.json` inside the Docker task) from the task-scoped Nomad Variable `nomad/jobs/catalog/catalog/catalog`. Create the variable first:

```sh
nomad var put nomad/jobs/catalog/catalog/catalog catalog_label=demo
nomad job run nomad/catalog.nomad.hcl
nomad var put nomad/jobs/catalog/catalog/catalog catalog_label=updated
```

The variable change triggers a file re-render and Nomad sends `SIGHUP` to the running container; it does not require a job resubmission. Check `/books` or the gRPC response before and after the update and verify the allocation remains the same. The `datacenters`, image tag, and service exposure depend on your cluster. There is no Nomad cluster in this repository, so the job is an example rather than a deployed integration test.
