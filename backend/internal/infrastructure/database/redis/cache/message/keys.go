package message_cache

import "fmt"

func ConversationSeqKeys(convId string) string {
	// TODO 修改 key
	return fmt.Sprintf("conversation:seq:%s", convId)
}
