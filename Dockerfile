FROM golang:1.25.1-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ai-agent .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/ai-agent .
COPY --from=builder /build/.env .
COPY --from=builder /build/prompts ./prompts
COPY --from=builder /build/data.json ./data.json

EXPOSE 8080

CMD ["./ai-agent"]

