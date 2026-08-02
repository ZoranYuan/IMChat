package model

type MessageAttachment struct {
	AttachmentId   string `gorm:"size:32;primaryKey" json:"attachmentId"`
	MessageId      string `gorm:"size:32;not null;index:idx_attachment_message" json:"messageId"`
	ConversationId string `gorm:"size:64;not null;index:idx_attachment_conversation" json:"conversationId"`
	FileId         string `gorm:"size:32;not null;index:idx_attachment_file" json:"fileId"`
	Kind           int8   `gorm:"not null;index:idx_attachment_kind" json:"kind"`
	ExpireAt       int64  `gorm:"not null;index:idx_attachment_expire_at" json:"expireAt"`
	CreatedAt      int64  `gorm:"not null;index:idx_attachment_created_at" json:"createdAt"`
}
