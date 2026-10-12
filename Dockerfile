FROM golang:1.24.10-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /catalog ./cmd/catalog

FROM alpine:3.22
COPY --from=build /catalog /catalog
COPY config.json /etc/catalog/config.json
USER 10001:10001
EXPOSE 8080 9090
ENTRYPOINT ["/catalog"]
CMD ["-config", "/etc/catalog/config.json"]
