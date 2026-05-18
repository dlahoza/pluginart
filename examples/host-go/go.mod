module example-host

go 1.26.2

require github.com/dlahoza/pluginart v0.1.0

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
)

replace github.com/dlahoza/pluginart => ../..
