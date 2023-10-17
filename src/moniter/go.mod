module moniter

go 1.21.0

require (
	common v0.0.0
	node v0.0.0
	github.com/cornelk/hashmap v1.0.8
	gopkg.in/yaml.v3 v3.0.1
)

replace common => ../common
replace node => ../node