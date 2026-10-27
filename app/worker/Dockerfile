FROM golang:1.27 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/worker /worker

ENV PORT=8000

EXPOSE 8000

ENTRYPOINT ["/worker"]
