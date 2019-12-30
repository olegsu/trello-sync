module github.com/olegsu/trello-sync

go 1.13

require (
	github.com/open-integration/core v0.14.0
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.6.1
	gopkg.in/ini.v1 v1.51.1 // indirect
)

replace gopkg.in/hairyhenderson/yaml.v2 => github.com/maxaudron/yaml v0.0.0-20190411130442-27c13492fe3

replace github.com/open-integration/core => ../../open-integration/core
