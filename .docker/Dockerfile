FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/homelab-dashboard ./cmd/homelab-dashboard

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

ENV PORT=8080
ENV ENABLE_KUBERNETES=true

COPY --from=build /out/homelab-dashboard /app/homelab-dashboard

EXPOSE 8080

ENTRYPOINT ["/app/homelab-dashboard"]
