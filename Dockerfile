FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /web-config-guard ./cmd/web-config-guard

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /web-config-guard /web-config-guard
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD ["/web-config-guard", "--healthcheck"]
ENTRYPOINT ["/web-config-guard", "--serve"]
