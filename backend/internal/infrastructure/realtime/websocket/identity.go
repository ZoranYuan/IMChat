package websocket

import (
	"errors"
	"strings"
)

var ErrInvalidSessionIdentity = errors.New("非法的会话身份")

type Platform string

const (
	PlatfromWeb Platform = "web"

	// 后期扩展
)

type SessionIdentity struct {
	Platform  Platform
	UserId    string
	SessionId string
	DeviceId  string
}

func NewSessionIdentity(userID, deviceID, platform, sessionID string) (SessionIdentity, error) {
	identity := SessionIdentity{
		UserId:    strings.TrimSpace(userID),
		DeviceId:  strings.TrimSpace(deviceID),
		Platform:  Platform(strings.ToLower(strings.TrimSpace(platform))),
		SessionId: strings.TrimSpace(sessionID),
	}

	if identity.UserId == "" || identity.DeviceId == "" || identity.SessionId == "" {
		return SessionIdentity{}, ErrInvalidSessionIdentity
	}

	switch identity.Platform {
	case PlatfromWeb:
		return identity, nil
	default:
		return SessionIdentity{}, ErrInvalidSessionIdentity
	}
}
