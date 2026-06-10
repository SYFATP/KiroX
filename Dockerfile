FROM node:22-bookworm AS frontend
WORKDIR /src
COPY frontend/package*.json ./frontend/
RUN npm --prefix frontend install
COPY frontend ./frontend
RUN npm --prefix frontend run build

FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/kirox .

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/kirox /app/kirox
RUN mkdir -p /data /output
ENV KIROX_WEB_PASSWORD=""
EXPOSE 8171
CMD ["/app/kirox", "--web", "--addr", "0.0.0.0:8171"]
