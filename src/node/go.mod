module mumu.com/node

go 1.21.0

require (
	github.com/cornelk/hashmap v1.0.8
	mumu.com/common v0.0.0-00010101000000-000000000000
)

require gopkg.in/yaml.v3 v3.0.1 // indirect

replace mumu.com/node => ../node

replace mumu.com/monitor => ../monitor

replace mumu.com/common => ../common
