#!/usr/bin/env node

import fs from "node:fs";
import process from "node:process";
import {
  AppStoreServerAPIClient,
  Environment,
  GetTransactionHistoryVersion,
  Order,
  SignedDataVerifier,
} from "@apple/app-store-server-library";

function readStdin() {
  return new Promise((resolve, reject) => {
    let input = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => {
      input += chunk;
    });
    process.stdin.on("end", () => resolve(input));
    process.stdin.on("error", reject);
  });
}

function environmentFromString(value) {
  const normalized = String(value || "").trim().toUpperCase();
  switch (normalized) {
    case "PRODUCTION":
      return Environment.PRODUCTION;
    case "SANDBOX":
      return Environment.SANDBOX;
    case "LOCAL_TESTING":
      return Environment.LOCAL_TESTING;
    default:
      throw new Error(`unsupported App Store Server API environment: ${value}`);
  }
}

function loadRootCertificates(paths) {
  if (!Array.isArray(paths) || paths.length === 0) {
    throw new Error("rootCertificatePaths is empty");
  }
  return paths.map((path) => fs.readFileSync(path));
}

function loadSigningKey(request) {
  const inlineKey = String(request.apiPrivateKey || "").trim();
  if (inlineKey) {
    return inlineKey;
  }

  const keyPath = String(request.apiPrivateKeyPath || "").trim();
  if (!keyPath) {
    throw new Error("apiPrivateKey or apiPrivateKeyPath is required");
  }
  return fs.readFileSync(keyPath, "utf8");
}

function compactString(value) {
  return String(value || "").trim();
}

function writeJSON(value) {
  process.stdout.write(`${JSON.stringify(value)}\n`);
}

function maxPagesFromRequest(request) {
  const value = Number(request.maxPages || 0);
  if (!Number.isFinite(value) || value <= 0) {
    return 10;
  }
  return Math.min(Math.trunc(value), 100);
}

async function decodeTransaction(verifier, signedTransactionJWS) {
  signedTransactionJWS = compactString(signedTransactionJWS);
  if (!signedTransactionJWS) {
    return undefined;
  }
  return verifier.verifyAndDecodeTransaction(signedTransactionJWS);
}

async function decodeRenewalInfo(verifier, signedRenewalInfoJWS) {
  signedRenewalInfoJWS = compactString(signedRenewalInfoJWS);
  if (!signedRenewalInfoJWS) {
    return undefined;
  }
  return verifier.verifyAndDecodeRenewalInfo(signedRenewalInfoJWS);
}

async function decodeNotification(verifier, signedPayload) {
  signedPayload = compactString(signedPayload);
  if (!signedPayload) {
    return undefined;
  }

  const notification = await verifier.verifyAndDecodeNotification(signedPayload);
  const signedTransactionInfo = compactString(notification?.data?.signedTransactionInfo);
  const signedRenewalInfo = compactString(notification?.data?.signedRenewalInfo);
  return {
    notification,
    transaction: signedTransactionInfo ? await decodeTransaction(verifier, signedTransactionInfo) : undefined,
    renewalInfo: signedRenewalInfo ? await decodeRenewalInfo(verifier, signedRenewalInfo) : undefined,
  };
}

async function getTransactionHistory(client, verifier, request) {
  const transactionId = compactString(request.transactionId);
  if (!transactionId) {
    throw new Error("transactionId is required");
  }

  const productIds = Array.isArray(request.productIds)
    ? request.productIds.map(compactString).filter(Boolean)
    : undefined;
  const historyRequest = {
    sort: Order.ASCENDING,
  };
  if (Number(request.startDate) > 0) {
    historyRequest.startDate = Number(request.startDate);
  }
  if (Number(request.endDate) > 0) {
    historyRequest.endDate = Number(request.endDate);
  }
  if (productIds?.length) {
    historyRequest.productIds = productIds;
  }

  const transactions = [];
  const pages = [];
  const maxPages = maxPagesFromRequest(request);
  let revision = compactString(request.revision) || null;
  let hasMore = true;

  for (let page = 0; hasMore && page < maxPages; page += 1) {
    const response = await client.getTransactionHistory(
      transactionId,
      revision,
      historyRequest,
      GetTransactionHistoryVersion.V2,
    );
    pages.push({
      revision: response.revision,
      hasMore: Boolean(response.hasMore),
      bundleId: response.bundleId,
      appAppleId: response.appAppleId,
      environment: response.environment,
      count: Array.isArray(response.signedTransactions) ? response.signedTransactions.length : 0,
    });

    for (const signedTransactionJWS of response.signedTransactions || []) {
      transactions.push({
        signedTransactionJWS,
        transaction: await decodeTransaction(verifier, signedTransactionJWS),
      });
    }

    revision = compactString(response.revision) || null;
    hasMore = Boolean(response.hasMore) && Boolean(revision);
  }

  return {
    pages,
    transactions,
    revision,
    hasMore,
  };
}

async function getSubscriptionStatus(client, verifier, request) {
  const transactionId = compactString(request.transactionId);
  if (!transactionId) {
    throw new Error("transactionId is required");
  }

  const response = await client.getAllSubscriptionStatuses(transactionId);
  const items = [];
  for (const group of response.data || []) {
    for (const item of group.lastTransactions || []) {
      const signedTransactionJWS = compactString(item.signedTransactionInfo);
      const signedRenewalInfoJWS = compactString(item.signedRenewalInfo);
      items.push({
        subscriptionGroupIdentifier: group.subscriptionGroupIdentifier,
        status: item.status || 0,
        originalTransactionId: item.originalTransactionId || "",
        signedTransactionJWS,
        signedRenewalInfoJWS,
        transaction: signedTransactionJWS ? await decodeTransaction(verifier, signedTransactionJWS) : undefined,
        renewalInfo: signedRenewalInfoJWS ? await decodeRenewalInfo(verifier, signedRenewalInfoJWS) : undefined,
      });
    }
  }

  return {
    environment: response.environment,
    bundleId: response.bundleId,
    appAppleId: response.appAppleId,
    items,
  };
}

async function getNotificationHistory(client, verifier, request) {
  const historyRequest = {};
  if (Number(request.startDate) > 0) {
    historyRequest.startDate = Number(request.startDate);
  }
  if (Number(request.endDate) > 0) {
    historyRequest.endDate = Number(request.endDate);
  }
  if (compactString(request.transactionId)) {
    historyRequest.transactionId = compactString(request.transactionId);
  }
  if (compactString(request.notificationType)) {
    historyRequest.notificationType = compactString(request.notificationType);
  }
  if (compactString(request.notificationSubtype)) {
    historyRequest.notificationSubtype = compactString(request.notificationSubtype);
  }
  if (typeof request.onlyFailures === "boolean") {
    historyRequest.onlyFailures = request.onlyFailures;
  }

  const items = [];
  const pages = [];
  const maxPages = maxPagesFromRequest(request);
  let paginationToken = compactString(request.paginationToken) || null;
  let hasMore = true;

  for (let page = 0; hasMore && page < maxPages; page += 1) {
    const response = await client.getNotificationHistory(paginationToken, historyRequest);
    pages.push({
      paginationToken: response.paginationToken,
      hasMore: Boolean(response.hasMore),
      count: Array.isArray(response.notificationHistory) ? response.notificationHistory.length : 0,
    });

    for (const item of response.notificationHistory || []) {
      const signedPayload = compactString(item.signedPayload);
      const decoded = signedPayload ? await decodeNotification(verifier, signedPayload) : {};
      items.push({
        signedPayload,
        sendAttempts: item.sendAttempts || [],
        notification: decoded.notification,
        transaction: decoded.transaction,
        renewalInfo: decoded.renewalInfo,
      });
    }

    paginationToken = compactString(response.paginationToken) || null;
    hasMore = Boolean(response.hasMore) && Boolean(paginationToken);
  }

  return {
    pages,
    notifications: items,
    paginationToken,
    hasMore,
  };
}

try {
  const raw = await readStdin();
  const request = JSON.parse(raw || "{}");
  const action = compactString(request.action);
  const bundleId = compactString(request.bundleId);
  const keyId = compactString(request.apiKeyId);
  const issuerId = compactString(request.apiIssuerId);
  if (!action) {
    throw new Error("action is required");
  }
  if (!bundleId) {
    throw new Error("bundleId is empty");
  }
  if (!keyId || !issuerId) {
    throw new Error("apiKeyId and apiIssuerId are required");
  }

  const environment = environmentFromString(request.environment);
  const signingKey = loadSigningKey(request);
  const rootCertificates = loadRootCertificates(request.rootCertificatePaths);
  const appAppleId = Number(request.appAppleId || 0) > 0 ? Number(request.appAppleId) : undefined;
  const client = new AppStoreServerAPIClient(signingKey, keyId, issuerId, bundleId, environment);
  const verifier = new SignedDataVerifier(
    rootCertificates,
    Boolean(request.enableOnlineChecks),
    environment,
    bundleId,
    appAppleId,
  );

  let data;
  switch (action) {
    case "transactionHistory":
      data = await getTransactionHistory(client, verifier, request);
      break;
    case "subscriptionStatus":
      data = await getSubscriptionStatus(client, verifier, request);
      break;
    case "notificationHistory":
      data = await getNotificationHistory(client, verifier, request);
      break;
    default:
      throw new Error(`unsupported action: ${action}`);
  }

  writeJSON({
    ok: true,
    action,
    data,
  });
} catch (error) {
  writeJSON({
    ok: false,
    error: error instanceof Error ? error.message : String(error),
    httpStatusCode: typeof error?.httpStatusCode === "number" ? error.httpStatusCode : undefined,
    apiError: typeof error?.apiError !== "undefined" ? error.apiError : undefined,
  });
  process.exitCode = 1;
}
