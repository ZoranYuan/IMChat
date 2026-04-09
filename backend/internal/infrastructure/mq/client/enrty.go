package mq_client

type Client interface {
	SendMessage(string, string, []byte) error
}
