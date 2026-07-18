package friend

import (
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	friendentity "IM_backend/internal/domain/friend/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
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

func (r *FriendRepository) WithTx(tx any) friendrepo.FriendRepository {
	return &FriendRepository{db: tx.(*gorm.DB)}
}

func (fr *FriendRepository) Create(domains []friendentity.Friend) error {
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

func (fr *FriendRepository) GetUserFriendList(userId string, status int) ([]friendentity.Friend, error) {
	var m []model.Friend

	if err := fr.db.
		Where("user_id = ? AND status != ?", userId, status).
		Find(&m).
		Error; err != nil {
		return nil, err
	}

	domains := make([]friendentity.Friend, 0, len(m))

	for _, r := range m {
		domains = append(domains, toDomain(r))
	}

	return domains, nil
}

func (fr *FriendRepository) FindRelation(userId, friendId string) (*friendentity.Friend, error) {
	var m model.Friend

	if err := fr.db.
		Where("user_id = ? AND friend_user_id = ?", userId, friendId).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	d := toDomain(m)
	return &d, nil
}
