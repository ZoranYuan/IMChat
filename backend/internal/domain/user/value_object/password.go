package user_valueobject

import "IM_backend/internal/infrastructure/pkg/encrypt"

type Password string

func (p Password) GenPasswordHash() (Password, error) {
	hp, err := encrypt.GenPasswordHash([]byte(p))
	return Password(hp), err
}

func (p Password) VertifyPasswordHash(hashed []byte) bool {
	return encrypt.VertifyPasswordHash([]byte(p), hashed)
}
