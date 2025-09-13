FROM golang:1.21 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/lester ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/lester /lester
EXPOSE 50051
ENV LESTER_PORT=50051
ENTRYPOINT ["/lester"]
