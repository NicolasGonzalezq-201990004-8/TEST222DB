FROM golang:1.21 AS build
WORKDIR /src
COPY . .     

ENV GOPROXY=https://proxy.golang.org,direct


WORKDIR /src/michael
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/michael ./cmd/client

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/michael /michael

ENTRYPOINT ["/michael"]