package session

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	attachTTLKey    = "_ttl"
	attachSaltKey   = "_salt"
	attachPubKeyKey = "_pk"
)

var (
	ErrTokenEmpty       = errors.New("token is empty")
	ErrTokenInvalid     = errors.New("token is invalid")
	ErrTokenExpired     = errors.New("token is expired")
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionAnonymous = errors.New("session is anonymous")
	ErrAttachNotFound   = errors.New("attach not found")
)

var SignSessionManager = NewSessionManager("spider-sign-session-secret", nil)

// TokenPayload 是写入 token 内部的基础信息。
// UID 表示用户 ID，ScopeID 表示作用域 ID，Salt 用于和服务端保存的 _salt 做二次校验，ExpiresAt 表示过期时间，IssuedAt 表示签发时间。
type TokenPayload struct {
	UID       uint64 `json:"uid"`
	ScopeID   uint64 `json:"scope_id"`
	Salt      string `json:"salt"`
	ExpiresAt int64  `json:"expires_at"`
	IssuedAt  int64  `json:"issued_at"`
}

// TokenService 负责 token 的生成、签名和解析。
// secret 是服务端签名密钥，用来生成和校验 HMAC-SHA256 签名。
type TokenService struct {
	secret []byte
}

// NewTokenService 创建 token 服务。
// secret: token 签名密钥，同一个服务端必须保持稳定；如果 secret 变了，旧 token 会全部失效。
func NewTokenService(secret string) *TokenService {
	return &TokenService{secret: []byte(secret)}
}

// NewToken 创建一个新 token。
// uid: 用户 ID。
// scopeID: 作用域 ID，可用于区分应用、区服、租户等场景。
// ttl: token 有效期；如果 <= 0，则默认 1 小时。
// 返回值 token 是给客户端保存和后续请求携带的令牌；salt 需要保存到服务端 SessionStore 的 _salt 中，用于后续校验 token 是否仍然有效。
func (s *TokenService) NewToken(uid uint64, scopeID uint64, ttl time.Duration) (token string, salt string, err error) {
	if ttl <= 0 {
		ttl = time.Hour
	}

	salt, err = randomHex(32)
	if err != nil {
		return "", "", err
	}

	now := time.Now()
	payload := TokenPayload{
		UID:       uid,
		ScopeID:   scopeID,
		Salt:      salt,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	token, err = s.encode(payload)
	if err != nil {
		return "", "", err
	}

	return token, salt, nil
}

// Parse 解析并校验 token 本身。
// token: 客户端传回来的 token 字符串。
// 只校验 token 签名和过期时间，不校验服务端保存的 _salt；完整会话校验请使用 SessionManager.FromToken。
func (s *TokenService) Parse(token string) (*TokenPayload, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrTokenEmpty
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrTokenInvalid
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	signBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	expectedSign := s.sign(payloadBytes)
	if !hmac.Equal(signBytes, expectedSign) {
		return nil, ErrTokenInvalid
	}

	payload := &TokenPayload{}
	if err := json.Unmarshal(payloadBytes, payload); err != nil {
		return nil, ErrTokenInvalid
	}

	if payload.ExpiresAt > 0 && time.Now().Unix() > payload.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return payload, nil
}

func (s *TokenService) encode(payload TokenPayload) (string, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	payloadPart := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signPart := base64.RawURLEncoding.EncodeToString(s.sign(payloadBytes))
	return payloadPart + "." + signPart, nil
}

func (s *TokenService) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

// SessionEntity 表示服务端保存的一份会话实体。
// UID 和 ScopeID 标识一个会话归属；Attach 保存 _salt、_ttl、_pk 以及业务自定义字段；ExpiresAt 表示内存会话过期时间。
type SessionEntity struct {
	UID       uint64
	ScopeID   uint64
	Attach    map[string]string
	ExpiresAt time.Time
}

func (e *SessionEntity) IsAnonymous() bool {
	return e == nil || e.UID == 0
}

func (e *SessionEntity) GetAttach(key string) (string, error) {
	if e == nil || e.Attach == nil {
		return "", ErrAttachNotFound
	}

	value, ok := e.Attach[checkCustomAttachKey(key)]
	if !ok {
		return "", ErrAttachNotFound
	}
	return value, nil
}

func (e *SessionEntity) GetAttachAsJSON(key string, out any) error {
	value, err := e.GetAttach(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(value), out)
}

// SessionStore 定义会话存储接口。 todo
// 现在内置 MemorySessionStore，后续可以实现 Redis/MySQL 版本。
type SessionStore interface {
	New(ctx context.Context, uid uint64, scopeID uint64, ttl time.Duration, attach map[string]string) error
	Put(ctx context.Context, uid uint64, scopeID uint64, ttl time.Duration, set map[string]string, remove []string) error
	Get(ctx context.Context, uid uint64, scopeID uint64, keys []string) (map[string]string, error)
	Delete(ctx context.Context, uid uint64, scopeID uint64) error
}

type MemorySessionStore struct {
	mu   sync.RWMutex
	data map[string]*SessionEntity
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{data: make(map[string]*SessionEntity)}
}

// New 新建一份会话数据。
// ctx: 请求上下文。
// uid: 用户 ID。
// scopeID: 作用域 ID。
// ttl: 会话有效期。
// attach: 会话附加字段，通常包含 _salt、_ttl，也可以包含业务字段。
func (s *MemorySessionStore) New(ctx context.Context, uid uint64, scopeID uint64, ttl time.Duration, attach map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	copiedAttach := cloneStringMap(attach)
	if ttl > 0 {
		copiedAttach[attachTTLKey] = strconv.FormatInt(int64(ttl.Seconds()), 10)
	}

	s.data[sessionKey(uid, scopeID)] = &SessionEntity{
		UID:       uid,
		ScopeID:   scopeID,
		Attach:    copiedAttach,
		ExpiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Put 修改已有会话的附加字段。
// ctx: 请求上下文。
// uid: 用户 ID。
// scopeID: 作用域 ID。
// ttl: 新的会话有效期；如果 > 0，会刷新过期时间。
// set: 需要新增或覆盖的 attach 字段。
// remove: 需要删除的 attach 字段名。
func (s *MemorySessionStore) Put(ctx context.Context, uid uint64, scopeID uint64, ttl time.Duration, set map[string]string, remove []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionKey(uid, scopeID)
	entity, ok := s.data[key]
	if !ok || entityExpired(entity) {
		delete(s.data, key)
		return ErrSessionNotFound
	}

	if entity.Attach == nil {
		entity.Attach = make(map[string]string)
	}
	for k, v := range set {
		entity.Attach[k] = v
	}
	for _, k := range remove {
		delete(entity.Attach, k)
	}
	if ttl > 0 {
		entity.Attach[attachTTLKey] = strconv.FormatInt(int64(ttl.Seconds()), 10)
		entity.ExpiresAt = time.Now().Add(ttl)
	}
	return nil
}

// Get 获取会话附加字段。
// ctx: 请求上下文。
// uid: 用户 ID。
// scopeID: 作用域 ID。
// keys: 需要读取的字段名；如果为空，则返回全部 attach。
func (s *MemorySessionStore) Get(ctx context.Context, uid uint64, scopeID uint64, keys []string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionKey(uid, scopeID)
	entity, ok := s.data[key]
	if !ok || entityExpired(entity) {
		delete(s.data, key)
		return nil, ErrSessionNotFound
	}

	if len(keys) == 0 {
		return cloneStringMap(entity.Attach), nil
	}

	result := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := entity.Attach[k]; ok {
			result[k] = v
		}
	}
	return result, nil
}

// Delete 删除会话，通常用于退出登录。
// ctx: 请求上下文。
// uid: 用户 ID。
// scopeID: 作用域 ID。
func (s *MemorySessionStore) Delete(ctx context.Context, uid uint64, scopeID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, sessionKey(uid, scopeID))
	return nil
}

// SessionManager 是对外使用的会话管理入口。
// 创建 token 用 NewToken / NewTokenWithTTL；根据 token 获取用户会话信息用 FromToken。
type SessionManager struct {
	tokenService *TokenService
	store        SessionStore
	defaultTTL   time.Duration
}

// NewSessionManager 创建会话管理器。
// secret: token 签名密钥。
// store: 会话存储实现；传 nil 时默认使用内存存储 MemorySessionStore。
func NewSessionManager(secret string, store SessionStore) *SessionManager {
	if store == nil {
		store = NewMemorySessionStore()
	}
	return &SessionManager{
		tokenService: NewTokenService(secret),
		store:        store,
		defaultTTL:   time.Hour,
	}
}

// SetDefaultTTL 设置默认 token / session 有效期。
// ttl: 默认有效期；只有 > 0 时才会生效。
func (m *SessionManager) SetDefaultTTL(ttl time.Duration) {
	if ttl > 0 {
		m.defaultTTL = ttl
	}
}

// NewAnonymous 创建匿名用户对象。
// 匿名用户 UID 为 0，调用 UID / ScopeID 等需要登录的方法会返回 ErrSessionAnonymous。
func (m *SessionManager) NewAnonymous() *SessionUser {
	return &SessionUser{manager: m, entity: &SessionEntity{Attach: map[string]string{}}}
}

// NewToken 创建 token，并同时在 SessionStore 中保存该 token 对应的会话信息。
// ctx: 请求上下文。
// uid: 用户 ID。
// scopeID: 作用域 ID。
// attach: 额外保存到会话里的业务字段，例如 device、role、serverId 等；自定义 key 不能以 _ 开头。
// 返回值 string 是 token；*SessionUser 是当前新建的会话用户对象。
func (m *SessionManager) NewToken(ctx context.Context, uid uint64, scopeID uint64, attach map[string]string) (string, *SessionUser, error) {
	return m.NewTokenWithTTL(ctx, uid, scopeID, m.defaultTTL, attach)
}

// NewTokenWithTTL 创建指定有效期的 token，并保存对应会话信息。
// ctx: 请求上下文。
// uid: 用户 ID。
// scopeID: 作用域 ID。
// ttl: 本次 token / session 的有效期。
// attach: 额外保存到会话里的业务字段。
// 使用场景：登录成功、注册成功后签发 token。
func (m *SessionManager) NewTokenWithTTL(ctx context.Context, uid uint64, scopeID uint64, ttl time.Duration, attach map[string]string) (string, *SessionUser, error) {
	token, salt, err := m.tokenService.NewToken(uid, scopeID, ttl)
	if err != nil {
		return "", nil, err
	}

	attachMap := cloneStringMap(attach)
	attachMap[attachSaltKey] = salt
	attachMap[attachTTLKey] = strconv.FormatInt(int64(ttl.Seconds()), 10)

	if err := m.store.New(ctx, uid, scopeID, ttl, attachMap); err != nil {
		return "", nil, err
	}

	user := &SessionUser{
		manager: m,
		entity: &SessionEntity{
			UID:       uid,
			ScopeID:   scopeID,
			Attach:    attachMap,
			ExpiresAt: time.Now().Add(ttl),
		},
	}
	return token, user, nil
}

// FromToken 根据客户端传回来的 token 恢复会话用户信息。
// ctx: 请求上下文。
// token: 客户端携带的 token。
// attachKeys: 除 _salt、_ttl、_pk 之外，额外希望读取的业务 attach 字段名。
// 返回值 *SessionUser 里可以通过 UID、ScopeID、GetAttachString 等方法拿到当前 token 对应的用户信息。
// 这是“token 创建完后，后续通过 token 拿相关信息”的主要方法。
func (m *SessionManager) FromToken(ctx context.Context, token string, attachKeys ...string) (*SessionUser, error) {
	payload, err := m.tokenService.Parse(token)
	if err != nil {
		return m.NewAnonymous(), err
	}

	keys := append([]string{attachSaltKey, attachTTLKey, attachPubKeyKey}, attachKeys...)
	attach, err := m.store.Get(ctx, payload.UID, payload.ScopeID, keys)
	if err != nil {
		return m.NewAnonymous(), err
	}

	if attach[attachSaltKey] != payload.Salt {
		return m.NewAnonymous(), ErrTokenInvalid
	}

	return &SessionUser{
		manager: m,
		entity: &SessionEntity{
			UID:     payload.UID,
			ScopeID: payload.ScopeID,
			Attach:  attach,
		},
	}, nil
}

// SessionUser 表示当前请求对应的会话用户。
// 可以通过 UID、ScopeID 获取身份信息，通过 GetAttachString 获取 token 关联的附加信息。
type SessionUser struct {
	manager *SessionManager
	entity  *SessionEntity
}

func (u *SessionUser) Check() error {
	if u == nil || u.entity == nil || u.entity.UID == 0 {
		return ErrSessionAnonymous
	}
	return nil
}

// UID 获取当前登录用户 ID。
// 如果当前是匿名用户，返回 ErrSessionAnonymous。
func (u *SessionUser) UID() (uint64, error) {
	if err := u.Check(); err != nil {
		return 0, err
	}
	return u.entity.UID, nil
}

// UIDOrDefault 获取用户 ID；如果未登录或为空，返回 0，不返回错误。
func (u *SessionUser) UIDOrDefault() uint64 {
	if u == nil || u.entity == nil {
		return 0
	}
	return u.entity.UID
}

// ScopeID 获取当前会话作用域 ID。
// 如果当前是匿名用户，返回 ErrSessionAnonymous。
func (u *SessionUser) ScopeID() (uint64, error) {
	if err := u.Check(); err != nil {
		return 0, err
	}
	return u.entity.ScopeID, nil
}

// Logout 删除当前用户的服务端会话，使该 token 后续无法通过 FromToken 校验。
// ctx: 请求上下文。
func (u *SessionUser) Logout(ctx context.Context) error {
	if err := u.Check(); err != nil {
		return err
	}
	return u.manager.store.Delete(ctx, u.entity.UID, u.entity.ScopeID)
}

// GetAttachString 获取当前 token / session 绑定的字符串附加信息。
// key: attach 字段名，自定义字段不能以 _ 开头。
func (u *SessionUser) GetAttachString(key string) (string, error) {
	if err := u.Check(); err != nil {
		return "", err
	}
	return u.entity.GetAttach(key)
}

func (u *SessionUser) GetAttachAsJSON(key string, out any) error {
	if err := u.Check(); err != nil {
		return err
	}
	return u.entity.GetAttachAsJSON(key, out)
}

// NewAttachSetter 创建 attach 修改器，用于修改当前会话绑定的附加信息。
func (u *SessionUser) NewAttachSetter() (*SessionAttachSetter, error) {
	if err := u.Check(); err != nil {
		return nil, err
	}
	return &SessionAttachSetter{
		user:   u,
		set:    make(map[string]string),
		remove: make([]string, 0),
	}, nil
}

type SessionAttachSetter struct {
	user   *SessionUser
	set    map[string]string
	remove []string
	ttl    time.Duration
}

// SetAttach 设置字符串 attach。
// key: 字段名，自定义字段不能以 _ 开头。
// value: 字段值；空字符串会转为删除该字段。
func (s *SessionAttachSetter) SetAttach(key string, value string) *SessionAttachSetter {
	key = checkCustomAttachKey(key)
	if value == "" {
		return s.Remove(key)
	}
	s.set[key] = value
	return s
}

func (s *SessionAttachSetter) SetAttachJSON(key string, value any) *SessionAttachSetter {
	bytesValue, err := json.Marshal(value)
	if err != nil || isEmptyJSON(string(bytesValue)) {
		return s.Remove(key)
	}
	return s.SetAttach(key, string(bytesValue))
}

func (s *SessionAttachSetter) Remove(key string) *SessionAttachSetter {
	s.remove = append(s.remove, checkCustomAttachKey(key))
	delete(s.set, key)
	return s
}

func (s *SessionAttachSetter) SetTTL(ttl time.Duration) *SessionAttachSetter {
	s.ttl = ttl
	return s
}

// Commit 提交 attach 修改，不生成新 token。
// ctx: 请求上下文。
func (s *SessionAttachSetter) Commit(ctx context.Context) error {
	user := s.user
	if err := user.Check(); err != nil {
		return err
	}

	if err := user.manager.store.Put(ctx, user.entity.UID, user.entity.ScopeID, s.ttl, s.set, s.remove); err != nil {
		return err
	}

	if user.entity.Attach == nil {
		user.entity.Attach = make(map[string]string)
	}
	for k, v := range s.set {
		user.entity.Attach[k] = v
	}
	for _, k := range s.remove {
		delete(user.entity.Attach, k)
	}
	if s.ttl > 0 {
		user.entity.Attach[attachTTLKey] = strconv.FormatInt(int64(s.ttl.Seconds()), 10)
	}
	return nil
}

// CommitAsNewToken 提交 attach 修改，并重新生成 token。
// ctx: 请求上下文。
// 返回值 string 是新 token；新 token 会刷新 _salt，旧 token 会因为 salt 不匹配而失效。
func (s *SessionAttachSetter) CommitAsNewToken(ctx context.Context) (string, error) {
	user := s.user
	if err := user.Check(); err != nil {
		return "", err
	}

	attach := cloneStringMap(user.entity.Attach)
	for k, v := range s.set {
		attach[k] = v
	}
	for _, k := range s.remove {
		delete(attach, k)
	}

	ttl := s.ttl
	if ttl <= 0 {
		ttl = user.manager.defaultTTL
	}

	token, newUser, err := user.manager.NewTokenWithTTL(ctx, user.entity.UID, user.entity.ScopeID, ttl, attach)
	if err != nil {
		return "", err
	}
	user.entity = newUser.entity
	return token, nil
}

func checkCustomAttachKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		panic("attach key is empty")
	}
	if strings.HasPrefix(key, "_") {
		panic(fmt.Sprintf("custom attach key not allow %s", key))
	}
	return key
}

func sessionKey(uid uint64, scopeID uint64) string {
	return strconv.FormatUint(uid, 10) + ":" + strconv.FormatUint(scopeID, 10)
}

func entityExpired(entity *SessionEntity) bool {
	return entity != nil && !entity.ExpiresAt.IsZero() && time.Now().After(entity.ExpiresAt)
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for k, v := range source {
		result[k] = v
	}
	return result
}

func randomHex(size int) (string, error) {
	bytesValue := make([]byte, size)
	if _, err := rand.Read(bytesValue); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytesValue), nil
}

func isEmptyJSON(value string) bool {
	trim := strings.TrimSpace(value)
	return trim == "" || trim == "{}" || trim == "[]" || trim == "null"
}
