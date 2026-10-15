# syntax=docker/dockerfile:1

# Build stage: compile a static binary.
FROM golang:1.26-alpine AS build

WORKDIR /src

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./lab5

# Run stage: minimal image without a shell, running as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/server"]
CMD ["-addr", ":8080"]
