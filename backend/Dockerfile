FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd

FROM alpine:3.24
RUN addgroup -S app && adduser -S -G app app
WORKDIR /srv
COPY --from=build /out/app ./app
USER app
EXPOSE 8080
CMD ["./app", "serve"]
