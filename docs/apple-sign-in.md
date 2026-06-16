# Sign in with Apple 接入说明

本文档用于约束和指导 spider-server 的 Sign in with Apple 后续改动。其他 AI 或开发者接手时，应先阅读本文，再修改登录、账号、删除账号、客户端对接相关代码。

## 当前配置

Apple Developer 后台已配置主 iOS App：

- Team ID: `XLVU7GGT6N`
- Key ID: `5LFYA472TZ`
- Client ID / Bundle ID: `hh.spider`
- Private Key: `AuthKey_5LFYA472TZ.p8`

`.p8` 私钥只能放在服务器安全路径，不能提交 Git，不能放进 iOS App 包。

服务器通过配置或环境变量读取 Apple 登录参数：

```bash
APPLE_TEAM_ID=XLVU7GGT6N
APPLE_KEY_ID=5LFYA472TZ
APPLE_CLIENT_ID=hh.spider
APPLE_PRIVATE_KEY_PATH=/secure/path/AuthKey_5LFYA472TZ.p8
APPLE_CLIENT_SECRET_TTL=24h
```

也可以在 `config.yaml` 的 `apple_sign_in` 节配置，但 `private_key_path` 推荐用环境变量注入。

## 服务端入口

Apple 登录统一走现有 `SignApi`：

```proto
rpc signInWithApple(AppleSignInRequest) returns (SignInResponse);
```

gRPC 方法名：

```text
/uc.SignApi/signInWithApple
```

请求字段：

```proto
message AppleSignInRequest {
  string identity_token = 1;
  string authorization_code = 2;
  string nonce = 3;
  string email = 4;
  string full_name = 5;
}
```

返回复用现有登录响应：

```proto
message SignInResponse {
  uint64 uid = 1;
  string uc_token = 2;
  string app_key = 11;
  string jwt_token = 12;
  string ws_url = 18;
}
```

客户端拿到 `uc_token` 后，按现有普通登录流程携带 `xx-token` 访问业务接口。

## iOS 客户端流程

客户端使用 `AuthenticationServices` 发起 Apple 登录：

1. 创建随机 nonce 原文。
2. 发起 `ASAuthorizationAppleIDRequest`。
3. 请求 scope: `.email`, `.fullName`。
4. 从回调中取：
   - `identityToken`
   - `authorizationCode`
   - nonce 原文
   - 首次登录可能返回的 `email`
   - 首次登录可能返回的 `fullName`
5. 调用 `/uc.SignApi/signInWithApple`。

注意：

- `fullName` 通常只在首次授权返回，服务端会首次保存。
- `email` 可能是真实邮箱，也可能是 `privaterelay.appleid.com` 隐藏邮箱。
- 不要在客户端直接信任 Apple 登录结果创建本地登录态，必须等服务器验证成功后使用服务器返回的 `uc_token`。

## Apple 登录字段说明

这些字段分成三类：客户端从 iOS 系统拿到的字段、`identityToken` 里的 claims、服务端向 Apple 换回来的 token 字段。业务登录最终仍然使用 spider-server 自己返回的 `uid` 和 `uc_token`。

### 客户端提交给服务端

- `identityToken`: Apple 返回给客户端的身份 JWT。它证明“Apple 确认了这个用户”。服务端必须验证它的签名、签发方、接收方、过期时间、`sub` 和 `nonce`，验证通过后才能信任里面的信息。
- `authorizationCode`: Apple 返回给客户端的一次性授权码。服务端可以拿它请求 Apple token endpoint，换取 `refresh_token`、`access_token`、`id_token` 等字段。这个 code 通常只能使用一次，不能当登录凭证长期保存。
- `nonce`: 客户端登录前生成的随机字符串，用来防重放和防串请求。客户端把 `sha256(nonce)` 放进 Apple 请求，服务端收到原始 `nonce` 后与 `identityToken` 里的 `nonce` 做匹配。
- `email`: 客户端首次授权时可能拿到的邮箱。只能作为资料保存，不能作为唯一账号标识。
- `fullName`: 客户端首次授权时可能拿到的姓名。只能作为资料保存，不能作为唯一账号标识。

### `identityToken` 中的重要 claims

- `iss`: issuer，签发方。Apple 登录必须是 `https://appleid.apple.com`，否则 token 不是 Apple 签发的。
- `aud`: audience，接收方。iOS App 登录时必须等于 Bundle ID，也就是当前配置的 `hh.spider`。这能防止别的 App 的 Apple token 被拿来登录本 App。
- `exp`: expiration time，过期时间。服务端必须拒绝过期的 `identityToken`。
- `sub`: subject，Apple 用户唯一标识。同一个 Apple 用户在同一个开发者 Team/App 下会有稳定的 `sub`。服务端必须用 `sub` 绑定本地用户，不要用邮箱绑定。
- `nonce`: Apple 写进 `identityToken` 的 nonce 值。当前实现支持匹配原文 nonce 或 `sha256(nonce)` 的 hex 值。
- `email`: Apple token 里可能携带的邮箱。用户隐藏邮箱时可能是 relay 邮箱。
- `email_verified`: Apple 对邮箱验证状态的声明。
- `is_private_email`: 是否为 Apple 隐藏邮箱。

### 服务端从 Apple token endpoint 换回来的字段

- `refresh_token`: 长期刷新凭证，后续撤销 Apple 授权或刷新 Apple token 时会用到。Apple 通常只在首次授权或特定场景返回，服务端拿到后应保存，不能返回客户端。
- `access_token`: Apple 访问令牌，生命周期较短。当前主要保存备用，不作为 spider-server 业务接口登录凭证。
- `id_token`: Apple token endpoint 返回的身份 JWT。它和客户端传来的 `identityToken` 都是 Apple 身份 JWT，当前服务端优先保存 token endpoint 返回的 `id_token`，没有时保存客户端传来的 `identityToken`。
- `expires_in`: `access_token` 的有效期，单位是秒。当前服务端用它计算 `TokenExpiresAt`。

字段关系简图：

```text
iOS AuthenticationServices
  -> identityToken / authorizationCode / nonce
  -> spider-server
  -> 校验 identityToken claims: iss / aud / exp / sub / nonce
  -> 用 authorizationCode 请求 Apple token endpoint
  -> refresh_token / access_token / id_token / expires_in
  -> AppleSub(sub) 绑定 users.id
  -> 返回 spider-server 自己的 uid / uc_token
```

## 服务端验证流程

当前实现位于：

- `game/appleauth/apple_auth.go`
- `game/router/sign_api.go`
- `mysql/model/apple_sign_in.go`
- `common/config/config.go`

服务端必须执行以下步骤：

1. 校验 `identity_token` 非空。
2. 解析 JWT header，要求 `alg == RS256` 且存在 `kid`。
3. 从 Apple JWKS 获取公钥：`https://appleid.apple.com/auth/keys`。
4. 校验 JWT 签名。
5. 校验 claims：
   - `iss == https://appleid.apple.com`
   - `aud == hh.spider`
   - 当前时间小于 `exp`
   - `sub` 非空
   - 如请求提供 nonce，则校验 nonce 原文或 nonce 的 sha256 hex。
6. 如 `authorization_code` 非空，用 `.p8` 生成 ES256 `client_secret`，调用 Apple token endpoint：
   - `POST https://appleid.apple.com/auth/token`
   - `grant_type=authorization_code`
7. 用 Apple `sub` 查找或创建本地用户。
8. 写入或更新 Apple 绑定表。
9. 调用 `session.SignSessionManager.NewToken` 返回现有 `uc_token`。

Apple `client_secret` 的 JWT 规则：

- header: `alg=ES256`, `kid=5LFYA472TZ`
- payload:
  - `iss=XLVU7GGT6N`
  - `sub=hh.spider`
  - `aud=https://appleid.apple.com`
  - `iat`
  - `exp`
- `exp` 最长不能超过 Apple 限制的 15777000 秒，当前默认 `24h`。

## 数据模型

Apple 账号绑定表模型是 `AppleSignInAccount`。

关键字段：

- `UserID`: 本地 `users.id`
- `AppleSub`: Apple 用户唯一标识，必须唯一
- `Email`: Apple token 或客户端首次透传的邮箱
- `EmailVerified`: Apple 邮箱验证状态
- `IsPrivateEmail`: 是否隐藏邮箱
- `FullName`: 首次登录时保存的名称
- `RefreshToken`: 用 `authorization_code` 换到的 refresh token
- `AccessToken`
- `AppleIDToken`
- `TokenExpiresAt`
- `LastLoginAt`

不要用邮箱作为 Apple 登录唯一键。必须用 `AppleSub` 绑定本地用户。

Apple 删除账号日志表模型是 `AppleSignInAccountDeletionLog`。当用户删除账号时，服务端会在硬删除 `AppleSignInAccount` 前，把原绑定记录完整复制到日志表。

日志表关键字段：

- `AppleSignInAccountID`: 原 `AppleSignInAccount.ID`
- `UserID`: 本地 `users.id`
- `AppleSub`
- `Email`
- `EmailVerified`
- `IsPrivateEmail`
- `FullName`
- `RefreshToken`
- `AccessToken`
- `AppleIDToken`
- `TokenExpiresAt`
- `LastLoginAt`
- `OriginalCreatedAt`
- `OriginalUpdatedAt`
- `OriginalDeletedAt`
- `RevokeAppleSignIn`: 删除账号时是否请求撤销 Apple 授权
- `DeleteReason`: 客户端传来的删除原因
- `DeletedAccountAt`: 删除账号发生时间

注意：日志表也保存 Apple token 快照，属于敏感数据，不能在接口中返回，不能随意导出。

本地自动生成账号格式：

```text
apple:{sha256(apple_sub)}
```

如果首次授权透传的 `fullName` 非空，Apple 登录成功后会用它初始化好友资料中的 `nickname`，前端通过 `FriendProfile.nickname` / `FriendListItem.nickname` / `FriendRequest.nickname` 展示；为空时继续使用默认昵称 `用户{uid}`。

## 错误码规范

登录 router 不能直接返回 `status.Error` / `status.Errorf`。业务失败必须使用：

```go
return session.Error(ctx, gamecode.SomeCode, &api.SignInResponse{})
```

Apple 登录相关错误码在 `game/code/error_code.go`：

- `10014` `SignAppleIdentityTokenEmpty`
- `10015` `SignAppleIdentityTokenInvalid`
- `10016` `SignAppleNonceInvalid`
- `10017` `SignAppleConfigInvalid`
- `10018` `SignAppleTokenExchangeFailed`
- `10019` `SignAppleAccountBindFailed`
- `10020` `SignAppleTokenRevokeFailed`

新增 Apple 登录错误时，继续使用 `100xx` 登录/auth 范围。

## 后续改动约束

修改 Apple 登录时必须遵守：

1. 不要把 `.p8`、私钥内容、真实服务器密钥提交到仓库。
2. 不要把 Apple 登录结果只放客户端验证。
3. 不要用 email 作为账号唯一键。
4. 不要绕过 `session.SignSessionManager.NewToken` 自建登录 token。
5. 不要在 router 里使用直接 gRPC error 作为业务错误。
6. 修改 proto 后必须运行 `./gen.sh`，并确认只保留相关生成文件差异。
7. 修改数据库模型后确认 `mysql/a_register_mysql.go` 已加入 AutoMigrate。
8. 完成后至少运行：

```bash
gofmt -w <changed-go-files>
go test ./...
git diff --check
rg -n "status\\.Error|status\\.Errorf|codes\\." game/router game/code
```

## 删除账号和撤销授权

当前删除账号接口已支持 Apple 授权撤销：

```proto
bool revoke_apple_sign_in = 3;
```

服务端实现规则：

1. 根据当前登录 `uc_token` 解析本地 `uid`。
2. 用 `uid` 查询 `AppleSignInAccount`。
3. 如果没有 Apple 绑定，直接继续本地删除账号。
4. 如果有 Apple 绑定且 `revoke_apple_sign_in == true`：
   - 优先使用 `RefreshToken`。
   - 没有 `RefreshToken` 时使用 `AccessToken`。
   - 生成 Apple `client_secret`。
   - 调用 Apple revoke endpoint：`POST https://appleid.apple.com/auth/revoke`。
5. 如果 Apple revoke 失败，删除流程中止并返回 `10020 SignAppleTokenRevokeFailed`。
6. 如果 Apple revoke 成功，或客户端没有要求 revoke，服务端都会先写入 `AppleSignInAccountDeletionLog` 全量快照，再硬删除本地 Apple 绑定记录，避免同一个 `apple_sub` 后续重新登录时命中已删除用户。
7. 将本地 `users.account` 改成 `del:{uid}:{timestamp}` 释放唯一账号名，再执行 GORM 软删除写入 `deleted_at`，最后退出当前 session。

注意：

- `refresh_token` 和 `access_token` 不能返回客户端。
- Apple revoke 需要 `.p8` 私钥配置可用，否则会返回 `10017 SignAppleConfigInvalid`。
- 本地 Apple 绑定记录使用硬删除，因为 `apple_sub` 有唯一索引，软删除不会释放唯一值。
- 本地 `users` 记录使用软删除，必须能在表里看到 `deleted_at` 被写入。

## 私有邮箱转发

如果产品要向用户发送邮件，且用户选择隐藏邮箱，需要在 Apple Developer 后台配置 Private Email Relay，并保证发信域名 SPF/DKIM 通过。

否则可以先只保存 `is_private_email` 和 relay email，不主动发邮件。

## 服务端通知

Apple 支持 server-to-server notification，用于接收：

- `email-disabled`
- `email-enabled`
- `consent-revoked`
- `account-deleted`

当前服务端还没有实现通知 endpoint。后续实现时应新建 HTTPS endpoint，校验 Apple JWS 签名后再更新 `AppleSignInAccount` 或本地用户状态。
