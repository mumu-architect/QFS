module node

go 1.21.0

require common v0.0.0

require Monitor v0.0.0

require (
	github.com/cornelk/hashmap v1.0.8 // indirect
)

replace Monitor => ../Monitor

replace common => ../common
