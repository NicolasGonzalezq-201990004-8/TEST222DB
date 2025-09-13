FROM golang:1.21 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/franklin ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/franklin /franklin
EXPOSE 50052
ENV FRANKLIN_PORT=50052
ENTRYPOINT ["/franklin"]
