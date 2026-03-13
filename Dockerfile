FROM golang:1.22-alpine AS build

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api

FROM alpine:3.19

RUN adduser -D -g '' app
USER app

COPY --from=build /bin/api /bin/api

EXPOSE 8080
ENV APP_PORT=:8080

ENTRYPOINT ["/bin/api"]
