package picture

import (
	"encoding/json"
	"time"
	"web_app2/internal/model/entity"
	resUser "web_app2/internal/model/response/user"
)

type PictureVO struct {
	ID             uint64         `json:"id,string"`
	URL            string         `json:"url"`
	ThumbnailURL   string         `json:"thumbnailUrl"`
	Name           string         `json:"name"`
	Introduction   string         `json:"introduction"`
	Category       string         `json:"category"`
	Tags           []string       `json:"tags" comment:"标签，将json转化为string数组"`
	PicSize        int64          `json:"picSize"`
	PicWidth       int            `json:"picWidth"`
	PicHeight      int            `json:"picHeight"`
	PicScale       float64        `json:"picScale"`
	PicFormat      string         `json:"picFormat"`
	UserID         uint64         `json:"userId,string", swaggertype:"string"`
	EditTime       time.Time      `json:"editTime"`
	CreateTime     time.Time      `json:"createTime"`
	UpdateTime     time.Time      `json:"updateTime"`
	User           resUser.UserVO `json:"user" comment:"用户信息"`
	SpaceID        uint64         `json:"spaceId,string" comment:"空间ID"`
	PicColor       string         `json:"picColor"`
	PermissionList []string       `json:"permissionList"` // 空间的权限列表
}

// 封装类转化为数据库对象
func VOToEntity(vo PictureVO) entity.Picture {
	//tags转化为json
	tags, _ := json.Marshal(vo.Tags)
	return entity.Picture{
		ID:           vo.ID,
		URL:          vo.URL,
		ThumbnailURL: vo.ThumbnailURL,
		Name:         vo.Name,
		Introduction: vo.Introduction,
		Category:     vo.Category,
		Tags:         string(tags),
		PicSize:      vo.PicSize,
		PicWidth:     vo.PicWidth,
		PicHeight:    vo.PicHeight,
		PicScale:     vo.PicScale,
		PicFormat:    vo.PicFormat,
		UserID:       vo.UserID,
		EditTime:     vo.EditTime,
		CreateTime:   vo.CreateTime,
		UpdateTime:   vo.UpdateTime,
		SpaceID:      vo.SpaceID,
		PicColor:     vo.PicColor,
	}
}

// 数据库对象转化为封装类
func EntityToVO(entity entity.Picture, userVO resUser.UserVO) PictureVO {
	//tags转化为数组
	var tags []string
	_ = json.Unmarshal([]byte(entity.Tags), &tags)
	return PictureVO{
		ID:           entity.ID,
		URL:          entity.URL,
		ThumbnailURL: entity.ThumbnailURL,
		Name:         entity.Name,
		Introduction: entity.Introduction,
		Category:     entity.Category,
		Tags:         tags,
		PicSize:      entity.PicSize,
		PicWidth:     entity.PicWidth,
		PicHeight:    entity.PicHeight,
		PicScale:     entity.PicScale,
		PicFormat:    entity.PicFormat,
		UserID:       entity.UserID,
		EditTime:     entity.EditTime,
		CreateTime:   entity.CreateTime,
		UpdateTime:   entity.UpdateTime,
		User:         userVO,
		SpaceID:      entity.SpaceID,
		PicColor:     entity.PicColor,
	}
}
