package user_valueobject

import "IM_backend/internal/infrastructure/crypto/encrypt"

type Password string

func (p Password) GenPasswordHash() (Password, error) {
	hp, err := encrypt.GenPasswordHash([]byte(p))
	return Password(hp), err
}

func (p Password) VerifyPasswordHash(hashed []byte) bool {
	return encrypt.VerifyPasswordHash([]byte(p), hashed)
}
