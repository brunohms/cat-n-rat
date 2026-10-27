FROM golang:1.27 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server

ENV PORT=8000 \
    RUN_MIGRATIONS=true

EXPOSE 8000

# Applies pending migrations before starting the API (set RUN_MIGRATIONS=false to skip)
ENTRYPOINT ["/server"]
