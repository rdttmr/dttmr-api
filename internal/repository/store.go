package repository

import "database/sql"

type Store struct {
	*Transactor
	Auth   *AuthRepo
	Invite *InviteRepo
	List   *ListRepo
	User   *UserRepo
}

func NewStore(db *sql.DB) *Store {
	t := NewTransactor(db)
	r := NewRepo(t)
	return &Store{
		Transactor: t,
		Auth:       &AuthRepo{r},
		Invite:     &InviteRepo{r},
		List:       &ListRepo{r},
		User:       &UserRepo{r},
	}
}
