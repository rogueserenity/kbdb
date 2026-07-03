FROM golang:1.26 AS build
WORKDIR /src
# Build context is the repo root (see template.yaml Metadata.DockerContext),
# so the whole module - including internal/ - is available here.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bootstrap ./functions/api

FROM public.ecr.aws/lambda/provided:al2023
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:1.0.1 /lambda-adapter /opt/extensions/lambda-adapter
COPY --from=build /bootstrap /var/task/bootstrap
ENTRYPOINT ["/var/task/bootstrap"]
