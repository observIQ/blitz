module github.com/observiq/blitz

go 1.26.0

tool github.com/mgechev/revive

tool (
	github.com/cpuguy83/go-md2man/v2
	github.com/securego/gosec/v2/cmd/gosec
)

require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/goccy/go-json v0.10.6
	github.com/google/uuid v1.6.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/prometheus/client_golang v1.24.1
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/spf13/viper v1.21.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/contrib/bridges/otelzap v0.20.1
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.22.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.46.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.46.0
	go.opentelemetry.io/otel/exporters/prometheus v0.68.0
	go.opentelemetry.io/otel/log v0.22.0
	go.opentelemetry.io/otel/log/logtest v0.22.0
	go.opentelemetry.io/otel/metric v1.46.0
	go.opentelemetry.io/otel/sdk v1.46.0
	go.opentelemetry.io/otel/sdk/log v0.22.0
	go.opentelemetry.io/otel/sdk/metric v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
	go.opentelemetry.io/proto/otlp v1.11.0
	go.uber.org/zap v1.28.0
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/sys v0.47.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.23.2 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	codeberg.org/chavacava/garif v0.2.0 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/anthropics/anthropic-sdk-go v1.66.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/buger/jsonparser v1.6.1 // indirect
	github.com/ccojocar/zxcvbn-go v1.0.4 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.20 // indirect
	github.com/googleapis/gax-go/v2 v2.24.0 // indirect
	github.com/gookit/color v1.6.1 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mgechev/dots v1.0.0 // indirect
	github.com/mgechev/revive v1.13.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openai/openai-go/v3 v3.52.0 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/securego/gosec/v2 v2.29.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/standard-webhooks/standard-webhooks/libraries v0.0.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/xo/terminfo v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.70.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.6 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/mod v0.40.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
	google.golang.org/api v0.293.0 // indirect
	google.golang.org/genai v1.69.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
)
