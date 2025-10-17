# build stage
FROM  --platform=$BUILDPLATFORM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=arm64
RUN go build -trimpath -ldflags="-s -w" -o /out/app .

# run stage (distroless or alpine)
FROM gcr.io/distroless/static-debian11:nonroot
WORKDIR /app
COPY --from=build /out/app ./app
USER nonroot:nonroot
EXPOSE 8000
ENTRYPOINT ["./app"]
