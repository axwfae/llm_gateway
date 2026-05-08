FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod ./

COPY . .
RUN go mod tidy

RUN go build -o /llm_gateway ./cmd/main.go

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /llm_gateway .
COPY config ./config

EXPOSE 18869 18866

CMD ["./llm_gateway", "-config", "config/config.yaml"]

# To enable debug mode:
# CMD ["./llm_gateway", "-config", "config/config.yaml", "-debug"]
# Or use environment variable:
# docker run -e DEBUG=true ...
