module node

go 1.21.0

require common v0.0.0

require moniter v0.0.0

require (
	github.com/cornelk/hashmap v1.0.8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace moniter => ../moniter

replace common => ../common
