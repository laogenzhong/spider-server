# Apple IAP verifier

This local Node module verifies StoreKit 2 transaction JWS values for the Go server.

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

Configure the Go server's `app_store.root_certificate_paths` with DER-encoded Apple root certificates from Apple PKI.
