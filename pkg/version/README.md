# Version Package

Il package `version` fornisce primitive riusabili per gestire la versione applicativa:

- metadata runtime (`service`, `version`, `commit`, `build_time`)
- supporto `-ldflags` per iniettare i valori in fase di build
- parsing e confronto [Semantic Versioning](https://semver.org/)

## Uso rapido

```go
import "github.com/nimburion/nimburion/pkg/version"

func main() {
	info := version.Current("orders-service")
	_ = info.String()
}
```

Build con metadata:

```bash
go build -ldflags "\
  -X github.com/nimburion/nimburion/pkg/version.AppVersion=v1.3.0 \
  -X github.com/nimburion/nimburion/pkg/version.GitCommit=$(git rev-parse --short HEAD) \
  -X github.com/nimburion/nimburion/pkg/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

