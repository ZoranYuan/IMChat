package message_cache

import "fmt"

func ConversationSeqKeys(convId string) string {
	return fmt.Sprintf("conversation:%s", convId)
}
