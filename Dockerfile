FROM golang:1.25 AS build-stage

WORKDIR /app

# 업스트림 gRPC 계약 모듈은 private 저장소가 공표한다(ADR-0020) — 프록시를 우회해 직접 받고,
# 토큰은 BuildKit secret으로 마운트한다(빌드 인자와 달리 레이어·history에 남지 않는다).
# 같은 RUN 안에서 gitconfig를 지우므로 토큰이 레이어에 남지 않는다.
ENV GOPRIVATE=github.com/jyoliar-careerhub/*

COPY go.mod go.sum ./
RUN --mount=type=secret,id=private_deps_token \
    set -eu; \
    if [ -s /run/secrets/private_deps_token ]; then \
      git config --global \
        url."https://x-access-token:$(cat /run/secrets/private_deps_token)@github.com/".insteadOf \
        "https://github.com/"; \
    fi; \
    go mod download; \
    rm -f /root/.gitconfig

COPY ./careerhub/apicomposer/ ./careerhub/apicomposer/
RUN ls --recursive ./

RUN CGO_ENABLED=0 GOOS=linux go build -o myapp ./careerhub/apicomposer/

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /app/myapp /myapp

USER nonroot:nonroot

ENTRYPOINT ["/myapp"]
