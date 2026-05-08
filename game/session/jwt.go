package session

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

const requestTokenTTL = 5 * time.Minute

var (
	ErrJWTPublicKeyEmpty      = errors.New("jwt public key is empty")
	ErrJWTPublicKeyInvalid    = errors.New("jwt public key is invalid")
	ErrJWTUserRangesTooMany   = errors.New("user ranges size must be less than 5")
	ErrJWTUserTagsTooMany     = errors.New("user tag k values size must be less than 5")
	ErrJWTUserRangeInvalid    = errors.New("user range is invalid")
	ErrJWTUserRangeDuplicated = errors.New("user range name is duplicated")
	ErrJWTUserTagInvalid      = errors.New("user tag k value is invalid")
	ErrJWTUserTagDuplicated   = errors.New("user tag k value name is duplicated")
	ErrJWTOptionsNotSupported = errors.New("token options is not supported")
)

// TokenJwtCreator 用于生成 gateway 请求 token。
// 对齐 Java 版本：JWE ECDH-ES + A128GCM 公钥加密。
// 生成结果是五段式 JWE Compact Token，只有持有对应私钥的一方才能解密读取 claims。
type TokenJwtCreator struct {
	publicKey *ecdsa.PublicKey
	now       func() time.Time
}

// NewTokenJwtCreator 创建 JWT 请求 token 生成器。
// publicKeyPEM: PEM 格式 EC 公钥字符串，用于执行 JWE ECDH-ES + A128GCM 加密。
func NewTokenJwtCreator(publicKeyPEM string) (*TokenJwtCreator, error) {
	publicKey, err := parseECDSAPublicKeyFromPEM(publicKeyPEM)
	if err != nil {
		return nil, err
	}

	return &TokenJwtCreator{
		publicKey: publicKey,
		now:       time.Now,
	}, nil
}

// MustNewTokenJwtCreator 创建 JWT 请求 token 生成器，失败时 panic。
// 适合在程序启动时用固定公钥初始化全局实例。
func MustNewTokenJwtCreator(secret string) *TokenJwtCreator {
	creator, err := NewTokenJwtCreator(secret)
	if err != nil {
		panic(err)
	}
	return creator
}

// TokenResult 是生成请求 token 后的返回结果。
// JWTToken: 给客户端携带的 JWT 字符串。
// URLPath: 形如 /s/{hash} 的路径，可用于 gateway 分流或路由。
type TokenResult struct {
	JWTToken string
	URLPath  string
}

// TokenOption 是生成请求 token 时的附加选项。
// HashValue: 如果 > 0，urlPath 使用 uid % HashValue；同时写入 JWT 的 hv claim。
// UserRanges: 用户范围列表，最多 4 个，会写入 JWT 的 ur claim；如果 HashValue <= 0，会用第一个 UserRange.ID 作为 hash 基础。
// UserTagKValues: 用户标签列表，最多 4 个，会写入 JWT 的 tkv claim。
// Options: Java 版本里暂未实现，当前必须为空。
type TokenOption struct {
	HashValue      int
	UserRanges     []UserRange
	UserTagKValues []UserTagKValue
	Options        string
}

// EmptyTokenOption 返回空选项。
func EmptyTokenOption() TokenOption {
	return TokenOption{}
}

// UserRange 表示用户范围标签。
// Name: 范围名，例如 server、region、tenant。
// ID: 范围 ID，必须大于 0。
type UserRange struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// UserTagKValue 表示用户自定义标签键值对。
// Name: 标签名。
// Value: 标签值。
type UserTagKValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// requestTokenClaims 是写入 JWT 的 claims。
type requestTokenClaims struct {
	UID       uint64          `json:"uid"`
	Scope     uint64          `json:"scope,omitempty"`
	HV        int             `json:"hv,omitempty"`
	Options   string          `json:"opt,omitempty"`
	UR        []UserRange     `json:"ur,omitempty"`
	TKV       []UserTagKValue `json:"tkv,omitempty"`
	ExpiresAt int64           `json:"exp"`
	IssuedAt  int64           `json:"iat"`
}

// GetRequestToken 根据 uid 创建请求 token。
// uid: 用户 ID。
func (c *TokenJwtCreator) GetRequestToken(uid uint64) (*TokenResult, error) {
	return c.GetRequestTokenWithScopeAndOption(uid, 0, EmptyTokenOption())
}

// GetRequestTokenWithScope 根据 uid 和 scopeID 创建请求 token。
// uid: 用户 ID。
// scopeID: 作用域 ID；如果 > 0，会写入 JWT 的 scope claim。
func (c *TokenJwtCreator) GetRequestTokenWithScope(uid uint64, scopeID uint64) (*TokenResult, error) {
	return c.GetRequestTokenWithScopeAndOption(uid, scopeID, EmptyTokenOption())
}

// GetRequestTokenWithOption 根据 uid 和选项创建请求 token。
// uid: 用户 ID。
// option: token 附加选项。
func (c *TokenJwtCreator) GetRequestTokenWithOption(uid uint64, option TokenOption) (*TokenResult, error) {
	return c.GetRequestTokenWithScopeAndOption(uid, 0, option)
}

// GetRequestTokenWithScopeAndOption 根据 uid、scopeID 和选项创建请求 token。
// uid: 用户 ID。
// scopeID: 作用域 ID；如果 > 0，会写入 JWT 的 scope claim。
// option: token 附加选项，控制 hv、ur、tkv 以及 urlPath 生成逻辑。
func (c *TokenJwtCreator) GetRequestTokenWithScopeAndOption(uid uint64, scopeID uint64, option TokenOption) (*TokenResult, error) {
	if err := validateTokenOption(option); err != nil {
		return nil, err
	}

	now := c.now().UTC()
	claims := requestTokenClaims{
		UID:       uid,
		ExpiresAt: now.Add(requestTokenTTL).Unix(),
		IssuedAt:  now.Unix(),
	}

	if scopeID > 0 {
		claims.Scope = scopeID
	}
	if option.HashValue > 0 {
		claims.HV = option.HashValue
	}
	if option.Options != "" {
		claims.Options = option.Options
	}
	if len(option.UserRanges) > 0 {
		claims.UR = option.UserRanges
	}
	if len(option.UserTagKValues) > 0 {
		claims.TKV = option.UserTagKValues
	}

	jwtToken, err := c.signJWT(claims)
	if err != nil {
		return nil, err
	}

	urlPath := buildURLPath(uid, option)
	return &TokenResult{
		JWTToken: jwtToken,
		URLPath:  urlPath,
	}, nil
}

// ParseRequestToken 解析 JWE token。
// 当前 TokenJwtCreator 只持有公钥，只负责加密生成 token，不能解密。
// 如果需要在 Go 里解析 token，需要另写持有私钥的 TokenJwtParser。
func (c *TokenJwtCreator) ParseRequestToken(token string) (*requestTokenClaims, error) {
	return nil, errors.New("jwe token needs private key to decrypt; use a private-key parser")
}

func (c *TokenJwtCreator) signJWT(claims requestTokenClaims) (string, error) {
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encrypter, err := jose.NewEncrypter(
		jose.A128GCM,
		jose.Recipient{
			Algorithm: jose.ECDH_ES,
			Key:       c.publicKey,
		},
		(&jose.EncrypterOptions{}).WithType("JWT"),
	)
	if err != nil {
		return "", err
	}

	object, err := encrypter.Encrypt(claimsBytes)
	if err != nil {
		return "", err
	}

	return object.CompactSerialize()
}

// parseECDSAPublicKeyFromPEM parses a PEM-encoded ECDSA public key.
func parseECDSAPublicKeyFromPEM(publicKeyPEM string) (*ecdsa.PublicKey, error) {
	publicKeyPEM = strings.TrimSpace(publicKeyPEM)
	if publicKeyPEM == "" {
		return nil, ErrJWTPublicKeyEmpty
	}

	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, ErrJWTPublicKeyInvalid
	}

	parsedKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJWTPublicKeyInvalid, err)
	}

	publicKey, ok := parsedKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: expected ECDSA public key, got %T", ErrJWTPublicKeyInvalid, parsedKey)
	}

	return publicKey, nil
}

func validateTokenOption(option TokenOption) error {
	if len(option.UserRanges) >= 5 {
		return ErrJWTUserRangesTooMany
	}
	if len(option.UserTagKValues) >= 5 {
		return ErrJWTUserTagsTooMany
	}
	if strings.TrimSpace(option.Options) != "" {
		return ErrJWTOptionsNotSupported
	}

	userRangeNames := make(map[string]struct{}, len(option.UserRanges))
	for _, userRange := range option.UserRanges {
		name := strings.TrimSpace(userRange.Name)
		if name == "" || userRange.ID <= 0 {
			return ErrJWTUserRangeInvalid
		}
		if _, exists := userRangeNames[name]; exists {
			return fmt.Errorf("%w: %s", ErrJWTUserRangeDuplicated, name)
		}
		userRangeNames[name] = struct{}{}
	}

	userTagNames := make(map[string]struct{}, len(option.UserTagKValues))
	for _, userTag := range option.UserTagKValues {
		name := strings.TrimSpace(userTag.Name)
		value := strings.TrimSpace(userTag.Value)
		if name == "" || value == "" {
			return ErrJWTUserTagInvalid
		}
		if _, exists := userTagNames[name]; exists {
			return fmt.Errorf("%w: %s", ErrJWTUserTagDuplicated, name)
		}
		userTagNames[name] = struct{}{}
	}

	return nil
}

func buildURLPath(uid uint64, option TokenOption) string {
	var hashStr string
	if option.HashValue > 0 {
		hashStr = strconv.FormatUint(uid%uint64(option.HashValue), 10)
	} else {
		hashValue := uid
		if len(option.UserRanges) > 0 {
			hashValue = uint64(option.UserRanges[0].ID)
		}
		hashStr = hashLongToPath(hashValue)
	}

	return "/s/" + hashStr
}

func hashLongToPath(value uint64) string {
	valueBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(valueBytes, value)

	sum := sha256.Sum256(valueBytes)
	hexValue := hex.EncodeToString(sum[:])

	// 对齐 Java: Hashing.sha256().hashBytes(Longs.toByteArray(hashValue)).toString().substring(8, 25)
	return hexValue[8:25]
}
