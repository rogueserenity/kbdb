FROM golang:1.26 AS build
WORKDIR /src
# Build context is the repo root (see docker-compose.yml), so the whole
# module is available here, same as functions/api/Dockerfile.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /mockoidc ./test/functional/support/mockoidc

FROM gcr.io/distroless/static-debian12
COPY --from=build /mockoidc /mockoidc
ENTRYPOINT ["/mockoidc"]
