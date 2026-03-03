package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/common/cache"
	"music-platform/internal/user/model"
	"music-platform/internal/user/repository"

	"github.com/go-redis/redis/v8"
)

const (
	defaultOnlineTTL          = 10 * time.Minute
	defaultHeartbeatIntervalS = 30
)

// UserService 用户服务接口
type UserService interface {
	Register(ctx context.Context, req *model.RegisterRequest) error
	Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error)
	AddMusic(ctx context.Context, req *model.AddMusicRequest) error
	TouchOnline(ctx context.Context, req *model.UserPingRequest) error
	StartOnlineSession(ctx context.Context, req *model.OnlineSessionStartRequest) (*model.OnlineSessionResponse, error)
	HeartbeatOnline(ctx context.Context, req *model.OnlineHeartbeatRequest) (*model.OnlineSessionResponse, error)
	GetOnlineStatus(ctx context.Context, req *model.OnlineStatusRequest) (*model.OnlineStatusResponse, error)
	LogoutOnline(ctx context.Context, req *model.OnlineLogoutRequest) error
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
	session, err := s.startOnlineSession(user.Account)
	if err == nil && session != nil {
		response.OnlineSessionToken = session.SessionToken
		response.OnlineHeartbeatIntervalS = session.HeartbeatIntervalSec
		response.OnlineTTLSec = session.OnlineTTLSec
	} else {
		s.markUserOnline(user.Account)
	}

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
		user, err := s.resolveUserByAccountOrUsername(ctx, "", req.Username)
		if err != nil {
			return err
		}
		account = user.Account
	}
	s.markUserOnline(account)
	return nil
}

// StartOnlineSession 创建在线会话（新客户端推荐）
func (s *userService) StartOnlineSession(ctx context.Context, req *model.OnlineSessionStartRequest) (*model.OnlineSessionResponse, error) {
	user, err := s.resolveUserByAccountOrUsername(ctx, req.Account, req.Username)
	if err != nil {
		return nil, err
	}
	return s.startOnlineSession(user.Account)
}

// HeartbeatOnline 在线心跳（需要会话 token）
func (s *userService) HeartbeatOnline(ctx context.Context, req *model.OnlineHeartbeatRequest) (*model.OnlineSessionResponse, error) {
	user, err := s.resolveUserByAccountOrUsername(ctx, req.Account, req.Username)
	if err != nil {
		return nil, err
	}
	account := user.Account
	token := strings.TrimSpace(req.SessionToken)
	if token == "" {
		return nil, errors.New("session_token 不能为空")
	}

	if err := s.ensureSessionTokenMatch(ctx, account, token); err != nil {
		return nil, err
	}
	return s.touchOnlineSession(account, token)
}

// GetOnlineStatus 查询在线状态（需要会话 token）
func (s *userService) GetOnlineStatus(ctx context.Context, req *model.OnlineStatusRequest) (*model.OnlineStatusResponse, error) {
	user, err := s.resolveUserByAccountOrUsername(ctx, req.Account, req.Username)
	if err != nil {
		return nil, err
	}
	account := user.Account
	token := strings.TrimSpace(req.SessionToken)
	if token == "" {
		return nil, errors.New("session_token 不能为空")
	}

	if err := s.ensureSessionTokenMatch(ctx, account, token); err != nil {
		return nil, err
	}

	rdb := cache.GetClient()
	if rdb == nil {
		return nil, errors.New("redis 未初始化")
	}
	keyAccount := onlineAccountKey(account)
	raw, err := rdb.Get(cache.GetContext(), keyAccount).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("查询在线状态失败: %w", err)
	}

	lastSeenAt := int64(0)
	if raw != "" {
		if ts, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil {
			lastSeenAt = ts
		}
	}

	ttlDur, err := rdb.TTL(cache.GetContext(), keyAccount).Result()
	if err != nil {
		return nil, fmt.Errorf("查询在线状态失败: %w", err)
	}
	ttlRemainingSec := int64(math.Max(0, ttlDur.Seconds()))

	return &model.OnlineStatusResponse{
		Account:              account,
		Online:               ttlRemainingSec > 0 && lastSeenAt > 0,
		LastSeenAt:           lastSeenAt,
		TTLRemainingSec:      ttlRemainingSec,
		HeartbeatIntervalSec: defaultHeartbeatIntervalS,
		OnlineTTLSec:         int(defaultOnlineTTL.Seconds()),
	}, nil
}

// LogoutOnline 主动下线（需要会话 token）
func (s *userService) LogoutOnline(ctx context.Context, req *model.OnlineLogoutRequest) error {
	user, err := s.resolveUserByAccountOrUsername(ctx, req.Account, req.Username)
	if err != nil {
		return err
	}
	account := user.Account
	token := strings.TrimSpace(req.SessionToken)
	if token == "" {
		return errors.New("session_token 不能为空")
	}

	if err := s.ensureSessionTokenMatch(ctx, account, token); err != nil {
		return err
	}

	rdb := cache.GetClient()
	if rdb == nil {
		return errors.New("redis 未初始化")
	}
	if err := rdb.Del(
		cache.GetContext(),
		onlineAccountKey(account),
		onlineSessionAccountKey(account),
		onlineSessionTokenKey(token),
	).Err(); err != nil {
		return fmt.Errorf("用户下线失败: %w", err)
	}
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
	key := onlineAccountKey(account)
	_ = cache.Set(key, strconv.FormatInt(time.Now().Unix(), 10), defaultOnlineTTL)
}

func (s *userService) resolveUserByAccountOrUsername(ctx context.Context, account string, username string) (*model.User, error) {
	account = strings.TrimSpace(account)
	username = strings.TrimSpace(username)
	if account == "" && username == "" {
		return nil, errors.New("account 或 username 至少提供一个")
	}

	if account != "" {
		user, err := s.userRepo.FindByAccount(ctx, account)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errors.New("用户不存在")
			}
			return nil, fmt.Errorf("查询用户失败: %w", err)
		}
		return user, nil
	}

	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return user, nil
}

func (s *userService) startOnlineSession(account string) (*model.OnlineSessionResponse, error) {
	token, err := generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("创建在线会话失败: %w", err)
	}
	return s.touchOnlineSession(account, token)
}

func (s *userService) touchOnlineSession(account string, token string) (*model.OnlineSessionResponse, error) {
	account = strings.TrimSpace(account)
	token = strings.TrimSpace(token)
	if account == "" || token == "" {
		return nil, errors.New("account 或 session_token 不能为空")
	}

	now := time.Now().Unix()
	expireAt := now + int64(defaultOnlineTTL.Seconds())
	rdb := cache.GetClient()
	if rdb == nil {
		return nil, errors.New("redis 未初始化")
	}

	pipe := rdb.TxPipeline()
	pipe.Set(cache.GetContext(), onlineAccountKey(account), strconv.FormatInt(now, 10), defaultOnlineTTL)
	pipe.Set(cache.GetContext(), onlineSessionAccountKey(account), token, defaultOnlineTTL)
	pipe.Set(cache.GetContext(), onlineSessionTokenKey(token), account, defaultOnlineTTL)
	if _, err := pipe.Exec(cache.GetContext()); err != nil {
		return nil, fmt.Errorf("更新在线状态失败: %w", err)
	}

	return &model.OnlineSessionResponse{
		Account:              account,
		SessionToken:         token,
		HeartbeatIntervalSec: defaultHeartbeatIntervalS,
		OnlineTTLSec:         int(defaultOnlineTTL.Seconds()),
		LastSeenAt:           now,
		ExpireAt:             expireAt,
	}, nil
}

func (s *userService) ensureSessionTokenMatch(ctx context.Context, account string, token string) error {
	rdb := cache.GetClient()
	if rdb == nil {
		return errors.New("redis 未初始化")
	}

	storedToken, err := rdb.Get(cache.GetContext(), onlineSessionAccountKey(account)).Result()
	if err == redis.Nil {
		return errors.New("在线会话不存在或已过期，请重新登录")
	}
	if err != nil {
		return fmt.Errorf("校验在线会话失败: %w", err)
	}
	if storedToken != token {
		return errors.New("在线会话无效，请重新登录")
	}

	storedAccount, err := rdb.Get(cache.GetContext(), onlineSessionTokenKey(token)).Result()
	if err == redis.Nil {
		return errors.New("在线会话不存在或已过期，请重新登录")
	}
	if err != nil {
		return fmt.Errorf("校验在线会话失败: %w", err)
	}
	if storedAccount != account {
		return errors.New("在线会话无效，请重新登录")
	}
	return nil
}

func onlineAccountKey(account string) string {
	return cache.PrefixUser + "online:account:" + account
}

func onlineSessionAccountKey(account string) string {
	return cache.PrefixUser + "online:session:account:" + account
}

func onlineSessionTokenKey(token string) string {
	return cache.PrefixUser + "online:session:token:" + token
}

func generateSessionToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
