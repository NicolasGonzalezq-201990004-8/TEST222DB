FROM golang:1.21 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/trevor ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/trevor /trevor
EXPOSE 50053
ENV TREVOR_PORT=50053
ENTRYPOINT ["/trevor"]
