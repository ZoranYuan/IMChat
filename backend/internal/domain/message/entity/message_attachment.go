package entity

type MessageAttachment struct {
	AttachmentId   string
	MessageId      string
	ConversationId string
	FileId         string
	Kind           int8
	ExpireAt       int64
	CreatedAt      int64
}

func NewMessageAttachment(
	attachmentId string,
	messageId string,
	conversationId string,
	fileId string,
	kind int8,
	expireAt int64,
	createdAt int64,
) *MessageAttachment {
	return &MessageAttachment{
		AttachmentId:   attachmentId,
		MessageId:      messageId,
		ConversationId: conversationId,
		FileId:         fileId,
		Kind:           kind,
		ExpireAt:       expireAt,
		CreatedAt:      createdAt,
	}
}
