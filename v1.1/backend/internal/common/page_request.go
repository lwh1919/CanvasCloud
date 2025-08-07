package common

type PageRequest struct {
	Current   int    `json:"current"`   //当前页数
	PageSize  int    `json:"pageSize"`  //页面大小
	SortField string `json:"sortField"` //排序字段
	SortOrder string `json:"sortOrder"` //排序顺序（默认升序）
}
