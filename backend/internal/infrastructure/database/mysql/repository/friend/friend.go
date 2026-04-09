package friend_repository

import (
	friend_entity "IM_backend/internal/domain/friend/entity"
	friend_repository_interface "IM_backend/internal/domain/friend/repository"
	"IM_backend/internal/infrastructure/database/mysql/model"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FriendRepository struct {
	db *gorm.DB
}

func NewFriendRepository(db *gorm.DB) *FriendRepository {
	return &FriendRepository{
		db: db,
	}
}

func (r *FriendRepository) WithTx(tx *gorm.DB) friend_repository_interface.FriendRepositoryInterface {
	return &FriendRepository{db: tx}
}

func (fr *FriendRepository) Create(domains []friend_entity.Friend) error {
	return fr.db.Transaction(func(tx *gorm.DB) error {
		for _, d := range domains {
			model := toModel(d)

			if err := tx.Clauses(clause.OnConflict{
				DoNothing: true,
			}).Create(&model).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (fr *FriendRepository) GetUserFriendList(userId string, status int) ([]friend_entity.Friend, error) {
	var m []model.Friend

	if err := fr.db.
		Where("user_id = ? AND status != ?", userId, status).
		Find(&m).
		Error; err != nil {
		return nil, err
	}

	domains := make([]friend_entity.Friend, 0, len(m))

	for _, r := range m {
		domains = append(domains, toDomain(r))
	}

	return domains, nil
}

func (fr *FriendRepository) FindRelation(userId, friendId string) (*friend_entity.Friend, error) {
	var m model.Friend

	if err := fr.db.
		Where("user_id = ? AND friend_id = ?", userId, friendId).
		Find(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	d := toDomain(m)
	return &d, nil
}
