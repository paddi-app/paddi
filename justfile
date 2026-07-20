build:
    go build -trimpath -ldflags "-s -w" -o bin/paddi main.go

build-staging:
    go build -trimpath -ldflags "-s -w -X github.com/paddi-app/paddi/internal/config.DefaultAPIBase=https://dev-api.paddi.app" -o bin/paddi-staging main.go
