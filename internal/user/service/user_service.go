package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/common/cache"
	"music-platform/internal/user/model"
	"music-platform/internal/user/repository"
)

// UserService 用户服务接口
type UserService interface {
	Register(ctx context.Context, req *model.RegisterRequest) error
	Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error)
	AddMusic(ctx context.Context, req *model.AddMusicRequest) error
	TouchOnline(ctx context.Context, req *model.UserPingRequest) error
}

type userService struct {
	userRepo      repository.UserRepository
	userMusicRepo repository.UserMusicRepository
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository, userMusicRepo repository.UserMusicRepository) UserService {
	return &userService{
		userRepo:      userRepo,
		userMusicRepo: userMusicRepo,
	}
}

// Register 用户注册
func (s *userService) Register(ctx context.Context, req *model.RegisterRequest) error {
	// 基本验证
	if req.Account == "" || req.Password == "" || req.Username == "" {
		return errors.New("账号、密码和用户名不能为空")
	}

	// 检查账号是否已存在
	existingUser, err := s.userRepo.FindByAccount(ctx, req.Account)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("查询用户失败: %w", err)
	}
	if existingUser != nil {
		return errors.New("该账号已被注册")
	}

	// 检查用户名是否已存在
	existingUsername, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("查询用户名失败: %w", err)
	}
	if existingUsername != nil {
		return errors.New("该用户名已被使用")
	}

	// 创建用户（注意：生产环境应该对密码进行哈希处理）
	user := &model.User{
		Account:  req.Account,
		Password: req.Password,
		Username: req.Username,
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		// 处理可能的数据库唯一约束错误
		if strings.Contains(err.Error(), "Duplicate entry") {
			if strings.Contains(err.Error(), "account") {
				return errors.New("该账号已被注册")
			}
			if strings.Contains(err.Error(), "username") {
				return errors.New("该用户名已被使用")
			}
			return errors.New("账号或用户名已存在")
		}
		return fmt.Errorf("创建用户失败: %w", err)
	}

	return nil
}

// Login 用户登录
func (s *userService) Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error) {
	// 检查缓存
	cacheKey := cache.PrefixUser + "login:" + req.Account + ":" + req.Password
	if cachedResult, err := cache.Get(cacheKey); err == nil && cachedResult != "" {
		// 缓存命中，解析返回
		response := &model.LoginResponse{}
		// 简化处理，实际应该用JSON解析
		return response, nil
	}

	// 查询用户
	user, err := s.userRepo.FindByAccount(ctx, req.Account)
	response := &model.LoginResponse{
		Success:      "false",
		SuccessBool:  false,
		Username:     "",
		SongPathList: []string{},
	}

	if err != nil {
		if err == sql.ErrNoRows {
			// 用户不存在
			return response, nil
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 验证密码（注意：生产环境应该使用哈希对比）
	if user.Password != req.Password {
		return response, nil
	}

	// 登录成功
	response.Success = "true"
	response.SuccessBool = true
	response.Username = user.Username
	s.markUserOnline(user.Account)

	// 获取用户收藏的音乐
	musicList, err := s.userMusicRepo.FindByUsername(ctx, user.Username)
	if err == nil {
		for _, um := range musicList {
			response.SongPathList = append(response.SongPathList, um.MusicPath)
		}
	}

	// 缓存成功的登录结果（5分钟）
	// cache.Set(cacheKey, response, cache.TTLShort)

	return response, nil
}

// TouchOnline 标记用户在线（用于客户端心跳）
func (s *userService) TouchOnline(ctx context.Context, req *model.UserPingRequest) error {
	account := strings.TrimSpace(req.Account)
	if account == "" {
		username := strings.TrimSpace(req.Username)
		if username == "" {
			return errors.New("account 或 username 至少提供一个")
		}
		user, err := s.userRepo.FindByUsername(ctx, username)
		if err != nil {
			if err == sql.ErrNoRows {
				return errors.New("用户不存在")
			}
			return fmt.Errorf("查询用户失败: %w", err)
		}
		account = user.Account
	}
	s.markUserOnline(account)
	return nil
}

// AddMusic 添加音乐到用户收藏
func (s *userService) AddMusic(ctx context.Context, req *model.AddMusicRequest) error {
	if req.Username == "" || req.MusicPath == "" {
		return errors.New("用户名和音乐路径不能为空")
	}

	// 查找用户
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("用户不存在")
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	// 添加收藏
	userMusic := &model.UserMusic{
		Username:  user.Username,
		MusicPath: req.MusicPath,
	}

	err = s.userMusicRepo.Create(ctx, userMusic)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return errors.New("该音乐已在收藏列表中")
		}
		return fmt.Errorf("添加收藏失败: %w", err)
	}

	return nil
}

func (s *userService) markUserOnline(account string) {
	account = strings.TrimSpace(account)
	if account == "" {
		return
	}
	key := cache.PrefixUser + "online:account:" + account
	ttl := 10 * time.Minute
	_ = cache.Set(key, strconv.FormatInt(time.Now().Unix(), 10), ttl)
}
