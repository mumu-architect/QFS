package qfsSdk

// 预上传响应
type PreUploadResp struct {
	RouteKey   string `json:"route_key"`
	LeaderAddr string `json:"leader_addr"`
}
