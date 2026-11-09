FROM golang:1.26-alpine AS builder

# A cached golang:1.26-alpine can be older than go.mod's go line: auto
# downloads the right toolchain instead of failing, as core's build does.
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# timetzdata builds the time zone database in, so the classes' time zones
# don't depend on the image having one.
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=readonly go build -trimpath -tags timetzdata -o /app ./cmd/app

# One image runs every command: `serve` (the default), `worker` and `migrate`.
# It is core's report image: serve renders the PDF reports with its
# JasperStarter, from the templates in /templates.
FROM ghcr.io/unlockedlabs/golang-jasper:latest

# Reported by /api/version. CI passes the release version (or sha-<short>),
# the commit and the branch; "unknown" for anyone who builds without them.
ARG APP_VERSION=unknown
ARG GIT_COMMIT=unknown
ARG GIT_BRANCH=unknown
ENV APP_VERSION=$APP_VERSION
ENV GIT_COMMIT=$GIT_COMMIT
ENV GIT_BRANCH=$GIT_BRANCH

# The base image already has an /app directory.
COPY --from=builder /app /programs
# The templates belong to the user serve runs as, so a report can recompile a
# template JasperStarter can't load and retry.
COPY --chown=65532:65532 templates /templates

# A nonroot user. Reports write their working files under /tmp.
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/programs"]
CMD ["serve"]
