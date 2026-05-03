package valueobject

type Status int

var (
	Friend      Status = 0 // 好友
	BlockOther  Status = 1 // 拉黑对方
	BeBlocked   Status = 2 // 被拉黑
	DeleteOther Status = 3 // 删除对方
	BeDeleted   Status = 4 // 被删除
)
