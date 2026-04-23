FROM golang:1.26 AS build

WORKDIR /src

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w \
    -X homelab-dashboard/internal/buildinfo.Version=${VERSION} \
    -X homelab-dashboard/internal/buildinfo.Commit=${COMMIT} \
    -X homelab-dashboard/internal/buildinfo.BuildDate=${BUILD_DATE}" \
    -o /out/homelab-dashboard ./cmd/homelab-dashboard

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

ENV PORT=8080
ENV ENABLE_KUBERNETES=true

COPY --from=build /out/homelab-dashboard /app/homelab-dashboard

EXPOSE 8080

ENTRYPOINT ["/app/homelab-dashboard"]
