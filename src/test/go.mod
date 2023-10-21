module test

go 1.21.0

require common v0.0.0

require Monitor v0.0.0

require (
	github.com/cornelk/hashmap v1.0.8 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace common => ../common

replace Monitor => ../Monitor
