package applications

import "IM_backend/internal/domain/user"

type UserApplication struct {
	userRepository user.UserRepoInterface
}

func NewUserApplication(userRepository user.UserRepoInterface) *UserApplication {
	return &UserApplication{
		userRepository: userRepository,
	}
}
