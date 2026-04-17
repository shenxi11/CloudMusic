package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	commentmodel "music-platform/internal/comment/model"
	commentrepo "music-platform/internal/comment/repository"
	usermodel "music-platform/internal/user/model"
)

type fakeCommentStore struct {
	catalog map[string]commentmodel.TrackMeta
	threads map[string]*commentmodel.CommentThread
	records map[int64]*commentmodel.CommentRecord
	nextID  int64
}

func newFakeCommentStore() *fakeCommentStore {
	return &fakeCommentStore{
		catalog: map[string]commentmodel.TrackMeta{},
		threads: map[string]*commentmodel.CommentThread{},
		records: map[int64]*commentmodel.CommentRecord{},
		nextID:  100,
	}
}

func (f *fakeCommentStore) ResolveCatalogTrack(musicPath string) (*commentmodel.TrackMeta, error) {
	meta, ok := f.catalog[musicPath]
	if !ok {
		return nil, commentmodel.ErrOnlineMusicOnly
	}
	return &meta, nil
}

func (f *fakeCommentStore) UpsertThread(meta commentmodel.TrackMeta) (*commentmodel.CommentThread, error) {
	if thread, ok := f.threads[meta.MusicPath]; ok {
		return thread, nil
	}
	thread := &commentmodel.CommentThread{
		ID:         int64(len(f.threads) + 1),
		MusicPath:  meta.MusicPath,
		Source:     meta.Source,
		SourceID:   meta.SourceID,
		MusicTitle: meta.MusicTitle,
		Artist:     meta.Artist,
	}
	f.threads[meta.MusicPath] = thread
	return thread, nil
}

func (f *fakeCommentStore) FindThreadByMusicPath(musicPath string) (*commentmodel.CommentThread, error) {
	thread, ok := f.threads[musicPath]
	if !ok {
		return nil, commentmodel.ErrCommentNotFound
	}
	return thread, nil
}

func (f *fakeCommentStore) GetCommentByID(commentID int64) (*commentmodel.CommentRecord, error) {
	record, ok := f.records[commentID]
	if !ok {
		return nil, commentmodel.ErrCommentNotFound
	}
	return record, nil
}

func (f *fakeCommentStore) ListRootComments(threadID int64, page, pageSize int) ([]commentmodel.CommentRecord, int, error) {
	items := []commentmodel.CommentRecord{}
	for _, record := range f.records {
		if record.ThreadID == threadID && record.RootCommentID == 0 {
			items = append(items, *record)
		}
	}
	return items, len(items), nil
}

func (f *fakeCommentStore) ListReplies(rootCommentID int64, page, pageSize int) ([]commentmodel.CommentRecord, int, error) {
	items := []commentmodel.CommentRecord{}
	for _, record := range f.records {
		if record.RootCommentID == rootCommentID {
			items = append(items, *record)
		}
	}
	return items, len(items), nil
}

func (f *fakeCommentStore) CreateRootComment(threadID int64, input commentrepo.CreateCommentInput) (*commentmodel.CommentRecord, error) {
	f.nextID++
	now := time.Now()
	record := &commentmodel.CommentRecord{
		ID:                 f.nextID,
		ThreadID:           threadID,
		UserAccount:        input.UserAccount,
		UsernameSnapshot:   input.UsernameSnapshot,
		AvatarPathSnapshot: input.AvatarPathSnapshot,
		Content:            input.Content,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	f.records[record.ID] = record
	thread := f.threadByID(threadID)
	thread.RootCommentCount++
	thread.TotalCommentCount++
	return record, nil
}

func (f *fakeCommentStore) CreateReply(threadID, rootCommentID int64, input commentrepo.CreateReplyInput) (*commentmodel.CommentRecord, error) {
	f.nextID++
	now := time.Now()
	replyTo := input.ReplyToCommentID
	record := &commentmodel.CommentRecord{
		ID:                 f.nextID,
		ThreadID:           threadID,
		RootCommentID:      rootCommentID,
		ReplyToCommentID:   &replyTo,
		UserAccount:        input.UserAccount,
		UsernameSnapshot:   input.UsernameSnapshot,
		AvatarPathSnapshot: input.AvatarPathSnapshot,
		ReplyToUserAccount: input.ReplyToUserAccount,
		ReplyToUsername:    input.ReplyToUsernameSnapshot,
		Content:            input.Content,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	f.records[record.ID] = record
	thread := f.threadByID(threadID)
	thread.TotalCommentCount++
	return record, nil
}

func (f *fakeCommentStore) SoftDeleteComment(commentID int64, userAccount string) (*commentmodel.CommentRecord, error) {
	record, ok := f.records[commentID]
	if !ok {
		return nil, commentmodel.ErrCommentNotFound
	}
	if record.UserAccount != userAccount {
		return nil, commentmodel.ErrDeleteForbidden
	}
	record.IsDeleted = true
	return record, nil
}

func (f *fakeCommentStore) threadByID(threadID int64) *commentmodel.CommentThread {
	for _, thread := range f.threads {
		if thread.ID == threadID {
			return thread
		}
	}
	return nil
}

type fakeUserRepo struct {
	users map[string]*usermodel.User
}

func (f *fakeUserRepo) Create(ctx context.Context, user *usermodel.User) error { return nil }
func (f *fakeUserRepo) FindByAccount(ctx context.Context, account string) (*usermodel.User, error) {
	user, ok := f.users[account]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return user, nil
}
func (f *fakeUserRepo) FindByID(ctx context.Context, id int) (*usermodel.User, error) {
	return nil, errors.New("not supported")
}
func (f *fakeUserRepo) FindByUsername(ctx context.Context, username string) (*usermodel.User, error) {
	for _, user := range f.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (f *fakeUserRepo) UpdateUsername(ctx context.Context, account string, oldUsername string, newUsername string) error {
	return nil
}
func (f *fakeUserRepo) UpdateAvatarPath(ctx context.Context, account string, avatarPath string) error {
	return nil
}

func TestListThreadCommentsWithoutThreadReturnsEmptyPayload(t *testing.T) {
	store := newFakeCommentStore()
	store.catalog["花海/花海.mp3"] = commentmodel.TrackMeta{
		MusicPath:  "花海/花海.mp3",
		Source:     "catalog",
		MusicTitle: "花海",
		Artist:     "周杰伦",
	}
	users := &fakeUserRepo{users: map[string]*usermodel.User{}}
	svc := NewCommentServiceWithSessionValidator(store, users, "http://127.0.0.1:8080", nil, nil, func(account, sessionToken string) error {
		return nil
	})

	resp, err := svc.ListThreadComments("花海/花海.mp3", 1, 20)
	if err != nil {
		t.Fatalf("ListThreadComments returned error: %v", err)
	}
	if resp.MusicTitle != "花海" || resp.RootCommentCount != 0 || len(resp.Items) != 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestCreateReplyUsesFlatThreadTarget(t *testing.T) {
	store := newFakeCommentStore()
	store.catalog["花海/花海.mp3"] = commentmodel.TrackMeta{
		MusicPath:  "花海/花海.mp3",
		Source:     "catalog",
		MusicTitle: "花海",
		Artist:     "周杰伦",
	}
	users := &fakeUserRepo{users: map[string]*usermodel.User{
		"u1": {Account: "u1", Username: "楼主", AvatarPath: "avatars/u1/avatar.jpg"},
		"u2": {Account: "u2", Username: "二楼"},
		"u3": {Account: "u3", Username: "三楼"},
	}}
	svc := NewCommentServiceWithSessionValidator(store, users, "http://127.0.0.1:8080", nil, nil, func(account, sessionToken string) error {
		return nil
	})

	rootResp, err := svc.CreateComment(context.Background(), commentmodel.CreateCommentRequest{
		UserAccount:  "u1",
		SessionToken: "ok",
		MusicPath:    "花海/花海.mp3",
		Content:      "主评论",
	})
	if err != nil {
		t.Fatalf("CreateComment returned error: %v", err)
	}

	firstReply, err := svc.CreateReply(context.Background(), rootResp.CommentID, commentmodel.CreateReplyRequest{
		UserAccount:  "u2",
		SessionToken: "ok",
		Content:      "回复楼主",
	})
	if err != nil {
		t.Fatalf("CreateReply returned error: %v", err)
	}

	secondReply, err := svc.CreateReply(context.Background(), rootResp.CommentID, commentmodel.CreateReplyRequest{
		UserAccount:     "u3",
		SessionToken:    "ok",
		Content:         "回复二楼",
		TargetCommentID: firstReply.CommentID,
	})
	if err != nil {
		t.Fatalf("CreateReply second returned error: %v", err)
	}
	if secondReply.Comment.RootCommentID != rootResp.CommentID {
		t.Fatalf("reply should stay under root comment, got %+v", secondReply.Comment)
	}
	if secondReply.Comment.ReplyTo == nil || secondReply.Comment.ReplyTo.CommentID != firstReply.CommentID {
		t.Fatalf("reply target mismatch: %+v", secondReply.Comment)
	}
	if secondReply.Comment.ReplyTo.Username != "二楼" {
		t.Fatalf("unexpected reply target username: %+v", secondReply.Comment.ReplyTo)
	}
}

func TestCreateCommentRejectsLocalPath(t *testing.T) {
	store := newFakeCommentStore()
	users := &fakeUserRepo{users: map[string]*usermodel.User{
		"u1": {Account: "u1", Username: "楼主"},
	}}
	svc := NewCommentServiceWithSessionValidator(store, users, "http://127.0.0.1:8080", nil, nil, func(account, sessionToken string) error {
		return nil
	})

	_, err := svc.CreateComment(context.Background(), commentmodel.CreateCommentRequest{
		UserAccount:  "u1",
		SessionToken: "ok",
		MusicPath:    "E:/Music/花海.mp3",
		Content:      "不应该成功",
	})
	if !errors.Is(err, commentmodel.ErrOnlineMusicOnly) {
		t.Fatalf("expected ErrOnlineMusicOnly, got %v", err)
	}
}
