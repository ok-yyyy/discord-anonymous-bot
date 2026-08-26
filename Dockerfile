# syntax=docker/dockerfile:1
# check=error=true

FROM golang:1.26 AS build

WORKDIR /app

RUN --mount=type=cache,target=/go/pkg/mod/ \
  --mount=type=bind,source=go.sum,target=go.sum \
  --mount=type=bind,source=go.mod,target=go.mod \
  go mod download -x

RUN --mount=type=cache,target=/go/pkg/mod/ \
  --mount=type=bind,target=. \
  CGO_ENABLED=0 go build -ldflags="-s" -trimpath -o /bin/app


FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app

COPY --from=build /bin/app /bin/app

ENV TZ=Asia/Tokyo

ENTRYPOINT ["/bin/app"]
