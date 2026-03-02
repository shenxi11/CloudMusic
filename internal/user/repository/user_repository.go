package repository

import (
	"context"
	"database/sql"
	"fmt"

	"music-platform/internal/user/model"
)

// UserRepository 用户仓储接口
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByAccount(ctx context.Context, account string) (*model.User, error)
	FindByID(ctx context.Context, id int) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
}

// userRepository 用户仓储实现
type userRepository struct {
	db *sql.DB
}

// NewUserRepository 创建用户仓储
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

// Create 创建用户
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	query := "INSERT INTO users (account, password, username) VALUES (?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, user.Account, user.Password, user.Username)
	return err
}

// FindByAccount 根据账号查找用户
func (r *userRepository) FindByAccount(ctx context.Context, account string) (*model.User, error) {
	user := &model.User{}
	query := "SELECT account, password, username FROM users WHERE account = ?"
	err := r.db.QueryRowContext(ctx, query, account).Scan(
		&user.Account, &user.Password, &user.Username,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// FindByID 根据ID查找用户（已废弃，保留接口兼容）
func (r *userRepository) FindByID(ctx context.Context, id int) (*model.User, error) {
	// users 表没有 id 字段，此方法不可用
	return nil, fmt.Errorf("FindByID not supported: users table has no id column")
}

// FindByUsername 根据用户名查找用户
func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	user := &model.User{}
	query := "SELECT account, password, username FROM users WHERE username = ?"
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.Account, &user.Password, &user.Username,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}
