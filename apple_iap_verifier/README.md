# Apple IAP verifier

This local Node module verifies StoreKit 2 transaction JWS values and App Store
Server Notifications V2 signed payloads for the Go server.

It uses Apple's official Node package, `@apple/app-store-server-library`, and exposes a small CLI:

```bash
npm install
node verify_transaction.mjs < request.json
```

Input JSON:

```json
{
  "signedTransactionJWS": "ey...",
  "bundleId": "hh.spider",
  "environment": "SANDBOX",
  "appAppleId": 0,
  "enableOnlineChecks": true,
  "rootCertificatePaths": ["/path/to/AppleRootCA-G3.cer"]
}
```

For App Store Server Notifications V2, pass the notification `signedPayload`:

```json
{
  "signedPayload": "ey...",
  "bundleId": "hh.spider",
  "environment": "SANDBOX",
  "appAppleId": 0,
  "enableOnlineChecks": true,
  "rootCertificatePaths": ["/path/to/AppleRootCA-G3.cer"]
}
```

Output JSON:

```json
{
  "ok": true,
  "transaction": {
    "transactionId": "...",
    "originalTransactionId": "...",
    "productId": "...",
    "bundleId": "hh.spider",
    "environment": "Sandbox"
  }
}
```

Notification output also includes the verified decoded notification and, when
present, the verified decoded `signedTransactionInfo` and `signedRenewalInfo`.

The Go server also uses `app_store_api.mjs` to call App Store Server API for
active reconciliation:

```bash
node app_store_api.mjs < request.json
```

Supported actions:

- `transactionHistory`
- `subscriptionStatus`
- `notificationHistory`

This script requires an App Store Server API key from App Store Connect:
`apiKeyId`, `apiIssuerId`, and either `apiPrivateKeyPath` or `apiPrivateKey`.
Do not assume a Sign in with Apple key can be reused for this API.

Configure the Go server's `app_store.root_certificate_paths` with DER-encoded Apple root certificates from Apple PKI.
