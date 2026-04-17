package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	commentmodel "music-platform/internal/comment/model"
	commentrepo "music-platform/internal/comment/repository"
	"music-platform/internal/common/cache"
	"music-platform/internal/common/eventbus"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/outbox"
	usermodel "music-platform/internal/user/model"
	userrepo "music-platform/internal/user/repository"
)

var (
	jamendoSourcePattern = regexp.MustCompile(`\[jamendo-([^\]]+)\]`)
	jamendoTitlePattern  = regexp.MustCompile(`\s*\[jamendo-[^\]]+\]\s*$`)
)

type CommentStore interface {
	ResolveCatalogTrack(musicPath string) (*commentmodel.TrackMeta, error)
	UpsertThread(meta commentmodel.TrackMeta) (*commentmodel.CommentThread, error)
	FindThreadByMusicPath(musicPath string) (*commentmodel.CommentThread, error)
	GetCommentByID(commentID int64) (*commentmodel.CommentRecord, error)
	ListRootComments(threadID int64, page, pageSize int) ([]commentmodel.CommentRecord, int, error)
	ListReplies(rootCommentID int64, page, pageSize int) ([]commentmodel.CommentRecord, int, error)
	CreateRootComment(threadID int64, input commentrepo.CreateCommentInput) (*commentmodel.CommentRecord, error)
	CreateReply(threadID, rootCommentID int64, input commentrepo.CreateReplyInput) (*commentmodel.CommentRecord, error)
	SoftDeleteComment(commentID int64, userAccount string) (*commentmodel.CommentRecord, error)
}

type SessionValidator func(account string, sessionToken string) error

type CommentService interface {
	ListThreadComments(musicPath string, page, pageSize int) (*commentmodel.ThreadCommentsResponse, error)
	ListReplies(rootCommentID int64, page, pageSize int) (*commentmodel.CommentRepliesResponse, error)
	CreateComment(ctx context.Context, req commentmodel.CreateCommentRequest) (*commentmodel.CommentActionResponse, error)
	CreateReply(ctx context.Context, rootCommentID int64, req commentmodel.CreateReplyRequest) (*commentmodel.CommentActionResponse, error)
	DeleteComment(ctx context.Context, commentID int64, req commentmodel.DeleteCommentRequest) (*commentmodel.CommentActionResponse, error)
}

type commentService struct {
	repo              CommentStore
	userRepo          userrepo.UserRepository
	baseURL           string
	publisher         eventbus.Publisher
	outbox            *outbox.Store
	validateSessionFn SessionValidator
}

func NewCommentService(
	repo CommentStore,
	userRepo userrepo.UserRepository,
	baseURL string,
	publisher eventbus.Publisher,
	outboxStore *outbox.Store,
) CommentService {
	return &commentService{
		repo:      repo,
		userRepo:  userRepo,
		baseURL:   strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		publisher: publisher,
		outbox:    outboxStore,
	}
}

func NewCommentServiceWithSessionValidator(
	repo CommentStore,
	userRepo userrepo.UserRepository,
	baseURL string,
	publisher eventbus.Publisher,
	outboxStore *outbox.Store,
	validateFn SessionValidator,
) CommentService {
	svc := NewCommentService(repo, userRepo, baseURL, publisher, outboxStore).(*commentService)
	svc.validateSessionFn = validateFn
	return svc
}

func (s *commentService) ListThreadComments(musicPath string, page, pageSize int) (*commentmodel.ThreadCommentsResponse, error) {
	page = normalizePage(page)
	pageSize = normalizePageSize(pageSize, 20, 100)

	meta, err := s.resolveTrackMeta(strings.TrimSpace(musicPath), "", "")
	if err != nil {
		return nil, err
	}

	thread, err := s.repo.FindThreadByMusicPath(meta.MusicPath)
	if err != nil {
		if errors.Is(err, commentmodel.ErrCommentNotFound) {
			return &commentmodel.ThreadCommentsResponse{
				MusicPath:         meta.MusicPath,
				Source:            meta.Source,
				SourceID:          meta.SourceID,
				MusicTitle:        meta.MusicTitle,
				Artist:            meta.Artist,
				CoverArtURL:       s.buildAssetURL(meta.CoverArtPath),
				RootCommentCount:  0,
				TotalCommentCount: 0,
				Page:              page,
				PageSize:          pageSize,
				Items:             []commentmodel.CommentItem{},
			}, nil
		}
		return nil, err
	}

	items, _, err := s.repo.ListRootComments(thread.ID, page, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &commentmodel.ThreadCommentsResponse{
		MusicPath:         thread.MusicPath,
		Source:            thread.Source,
		SourceID:          thread.SourceID,
		MusicTitle:        firstNonEmpty(thread.MusicTitle, meta.MusicTitle),
		Artist:            firstNonEmpty(thread.Artist, meta.Artist),
		CoverArtURL:       s.buildAssetURL(firstNonEmpty(thread.CoverArtPath, meta.CoverArtPath)),
		RootCommentCount:  thread.RootCommentCount,
		TotalCommentCount: thread.TotalCommentCount,
		Page:              page,
		PageSize:          pageSize,
		Items:             make([]commentmodel.CommentItem, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, s.toCommentItem(item))
	}
	return resp, nil
}

func (s *commentService) ListReplies(rootCommentID int64, page, pageSize int) (*commentmodel.CommentRepliesResponse, error) {
	page = normalizePage(page)
	pageSize = normalizePageSize(pageSize, 50, 100)

	root, err := s.repo.GetCommentByID(rootCommentID)
	if err != nil {
		return nil, err
	}
	if root.RootCommentID != 0 {
		return nil, commentmodel.ErrRootCommentRequired
	}

	items, total, err := s.repo.ListReplies(rootCommentID, page, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &commentmodel.CommentRepliesResponse{
		RootCommentID: rootCommentID,
		Page:          page,
		PageSize:      pageSize,
		Total:         total,
		Items:         make([]commentmodel.CommentItem, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, s.toCommentItem(item))
	}
	return resp, nil
}

func (s *commentService) CreateComment(ctx context.Context, req commentmodel.CreateCommentRequest) (*commentmodel.CommentActionResponse, error) {
	user, err := s.validateUserSession(ctx, req.UserAccount, req.SessionToken)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("评论内容不能为空")
	}

	meta, err := s.resolveTrackMeta(req.MusicPath, req.MusicTitle, req.Artist)
	if err != nil {
		return nil, err
	}

	thread, err := s.repo.UpsertThread(*meta)
	if err != nil {
		return nil, err
	}

	record, err := s.repo.CreateRootComment(thread.ID, commentrepo.CreateCommentInput{
		UserAccount:        user.Account,
		UsernameSnapshot:   user.Username,
		AvatarPathSnapshot: user.AvatarPath,
		Content:            content,
	})
	if err != nil {
		return nil, err
	}

	s.publishEvent("music.comment.created", map[string]interface{}{
		"music_path":   thread.MusicPath,
		"thread_id":    thread.ID,
		"comment_id":   record.ID,
		"user_account": user.Account,
		"is_reply":     false,
	})

	return &commentmodel.CommentActionResponse{
		Success:   true,
		Message:   "评论成功",
		CommentID: record.ID,
		Comment:   s.toCommentItem(*record),
	}, nil
}

func (s *commentService) CreateReply(ctx context.Context, rootCommentID int64, req commentmodel.CreateReplyRequest) (*commentmodel.CommentActionResponse, error) {
	user, err := s.validateUserSession(ctx, req.UserAccount, req.SessionToken)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("回复内容不能为空")
	}

	root, err := s.repo.GetCommentByID(rootCommentID)
	if err != nil {
		return nil, err
	}
	if root.RootCommentID != 0 {
		return nil, commentmodel.ErrRootCommentRequired
	}

	target := root
	if req.TargetCommentID > 0 {
		target, err = s.repo.GetCommentByID(req.TargetCommentID)
		if err != nil {
			return nil, err
		}
		if target.ThreadID != root.ThreadID {
			return nil, commentmodel.ErrReplyTargetInvalid
		}
		if target.ID != root.ID && target.RootCommentID != root.ID {
			return nil, commentmodel.ErrReplyTargetInvalid
		}
	}

	record, err := s.repo.CreateReply(root.ThreadID, root.ID, commentrepo.CreateReplyInput{
		CreateCommentInput: commentrepo.CreateCommentInput{
			UserAccount:        user.Account,
			UsernameSnapshot:   user.Username,
			AvatarPathSnapshot: user.AvatarPath,
			Content:            content,
		},
		ReplyToCommentID:        target.ID,
		ReplyToUserAccount:      target.UserAccount,
		ReplyToUsernameSnapshot: target.UsernameSnapshot,
	})
	if err != nil {
		return nil, err
	}

	s.publishEvent("music.comment.created", map[string]interface{}{
		"thread_id":         root.ThreadID,
		"root_comment_id":   root.ID,
		"comment_id":        record.ID,
		"user_account":      user.Account,
		"target_comment_id": target.ID,
		"is_reply":          true,
	})

	return &commentmodel.CommentActionResponse{
		Success:   true,
		Message:   "回复成功",
		CommentID: record.ID,
		Comment:   s.toCommentItem(*record),
	}, nil
}

func (s *commentService) DeleteComment(ctx context.Context, commentID int64, req commentmodel.DeleteCommentRequest) (*commentmodel.CommentActionResponse, error) {
	user, err := s.validateUserSession(ctx, req.UserAccount, req.SessionToken)
	if err != nil {
		return nil, err
	}

	record, err := s.repo.SoftDeleteComment(commentID, user.Account)
	if err != nil {
		return nil, err
	}

	s.publishEvent("music.comment.deleted", map[string]interface{}{
		"thread_id":    record.ThreadID,
		"comment_id":   record.ID,
		"user_account": user.Account,
	})

	return &commentmodel.CommentActionResponse{
		Success:   true,
		Message:   "删除成功",
		CommentID: record.ID,
		Comment:   s.toCommentItem(*record),
	}, nil
}

func (s *commentService) validateUserSession(ctx context.Context, account string, sessionToken string) (*usermodel.User, error) {
	account = strings.TrimSpace(account)
	sessionToken = strings.TrimSpace(sessionToken)
	if account == "" {
		return nil, fmt.Errorf("user_account 不能为空")
	}
	if sessionToken == "" {
		return nil, fmt.Errorf("session_token 不能为空")
	}

	user, err := s.userRepo.FindByAccount(ctx, account)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	validateFn := s.validateSessionFn
	if validateFn == nil {
		validateFn = defaultSessionValidator
	}
	if err := validateFn(account, sessionToken); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *commentService) resolveTrackMeta(musicPath string, musicTitle string, artist string) (*commentmodel.TrackMeta, error) {
	canonical := normalizeMusicPath(musicPath)
	if canonical == "" {
		return nil, fmt.Errorf("music_path 不能为空")
	}

	if sourceID, ok := parseJamendoSourceID(canonical); ok {
		title := strings.TrimSpace(musicTitle)
		if title == "" {
			title = deriveJamendoTitle(canonical)
		}
		return &commentmodel.TrackMeta{
			MusicPath:  canonical,
			Source:     "jamendo",
			SourceID:   sourceID,
			MusicTitle: title,
			Artist:     strings.TrimSpace(artist),
		}, nil
	}

	meta, err := s.repo.ResolveCatalogTrack(canonical)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(meta.MusicTitle) == "" {
		meta.MusicTitle = strings.TrimSpace(musicTitle)
	}
	if strings.TrimSpace(meta.Artist) == "" {
		meta.Artist = strings.TrimSpace(artist)
	}
	return meta, nil
}

func (s *commentService) toCommentItem(record commentmodel.CommentRecord) commentmodel.CommentItem {
	item := commentmodel.CommentItem{
		CommentID:     record.ID,
		RootCommentID: record.RootCommentID,
		UserAccount:   record.UserAccount,
		Username:      record.UsernameSnapshot,
		AvatarURL:     s.buildAssetURL(record.AvatarPathSnapshot),
		Content:       strings.TrimSpace(record.Content),
		IsDeleted:     record.IsDeleted,
		CreatedAt:     record.CreatedAt.Format("2006-01-02 15:04:05"),
		ReplyCount:    record.ReplyCount,
	}
	if record.IsDeleted {
		item.Content = "该评论已删除"
		item.AvatarURL = ""
	}
	if record.ReplyToCommentID != nil && strings.TrimSpace(record.ReplyToUsername) != "" {
		item.ReplyTo = &commentmodel.CommentReplyTo{
			CommentID:   *record.ReplyToCommentID,
			UserAccount: record.ReplyToUserAccount,
			Username:    record.ReplyToUsername,
		}
	}
	return item
}

func (s *commentService) buildAssetURL(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	trimmed = strings.TrimPrefix(trimmed, "/")
	if s.baseURL == "" {
		return "/uploads/" + trimmed
	}
	return s.baseURL + "/uploads/" + trimmed
}

func (s *commentService) publishEvent(eventType string, payload interface{}) {
	if s.publisher == nil && s.outbox == nil {
		return
	}

	evt, err := eventbus.NewEvent(eventType, "profile-service", payload)
	if err != nil {
		logger.Warn("创建评论事件失败: %v", err)
		return
	}
	if s.publisher != nil {
		if err := s.publisher.Publish(context.Background(), evt); err == nil {
			return
		} else {
			logger.Warn("发布评论事件失败，将写入 outbox: %v", err)
			s.enqueueOutbox(evt, err.Error())
			return
		}
	}
	s.enqueueOutbox(evt, "publisher_unavailable")
}

func (s *commentService) enqueueOutbox(evt *eventbus.Event, reason string) {
	if s.outbox == nil || evt == nil {
		return
	}
	if err := s.outbox.SavePending(evt, reason); err != nil {
		logger.Warn("写入评论 outbox 失败: %v", err)
	}
}

func defaultSessionValidator(account string, sessionToken string) error {
	rdb := cache.GetClient()
	if rdb == nil {
		return commentmodel.ErrAuthUnavailable
	}
	storedAccount, err := rdb.Get(cache.GetContext(), onlineSessionTokenKey(sessionToken)).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return commentmodel.ErrInvalidSession
		}
		return commentmodel.ErrAuthUnavailable
	}
	if storedAccount != account {
		return commentmodel.ErrInvalidSession
	}
	return nil
}

func onlineSessionTokenKey(token string) string {
	return cache.PrefixUser + "online:session:token:" + token
}

func normalizeMusicPath(musicPath string) string {
	trimmed := strings.TrimSpace(musicPath)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, "/uploads/"); idx >= 0 {
		trimmed = trimmed[idx+len("/uploads/"):]
	}
	return strings.TrimSpace(trimmed)
}

func parseJamendoSourceID(musicPath string) (string, bool) {
	matches := jamendoSourcePattern.FindStringSubmatch(musicPath)
	if len(matches) < 2 {
		return "", false
	}
	return strings.TrimSpace(matches[1]), true
}

func deriveJamendoTitle(musicPath string) string {
	base := musicPath
	if dot := strings.LastIndex(base, "."); dot > 0 {
		base = base[:dot]
	}
	base = jamendoTitlePattern.ReplaceAllString(base, "")
	return strings.TrimSpace(base)
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int, fallback int, max int) int {
	if pageSize <= 0 {
		pageSize = fallback
	}
	if pageSize > max {
		return max
	}
	return pageSize
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
