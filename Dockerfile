FROM golang:1.25 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./careerhub/apicomposer/ ./careerhub/apicomposer/
RUN ls --recursive ./

RUN CGO_ENABLED=0 GOOS=linux go build -o myapp ./careerhub/apicomposer/

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /app/myapp /myapp

USER nonroot:nonroot

ENTRYPOINT ["/myapp"]
