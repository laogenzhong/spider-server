#!/usr/bin/env node

import fs from "node:fs";
import process from "node:process";
import { Environment, SignedDataVerifier } from "@apple/app-store-server-library";

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
    case "XCODE":
      return Environment.XCODE;
    case "LOCAL_TESTING":
      return Environment.LOCAL_TESTING;
    default:
      throw new Error(`unsupported App Store environment: ${value}`);
  }
}

function loadRootCertificates(paths) {
  if (!Array.isArray(paths) || paths.length === 0) {
    throw new Error("rootCertificatePaths is empty");
  }
  return paths.map((path) => fs.readFileSync(path));
}

function writeJSON(value) {
  process.stdout.write(`${JSON.stringify(value)}\n`);
}

try {
  const raw = await readStdin();
  const request = JSON.parse(raw || "{}");
  const signedPayload = String(request.signedPayload || request.signedNotificationPayload || "").trim();
  const signedTransaction = String(request.signedTransactionJWS || "").trim();

  const environment = environmentFromString(request.environment);
  const rootCertificates = loadRootCertificates(request.rootCertificatePaths);
  const bundleId = String(request.bundleId || "").trim();
  if (!bundleId) {
    throw new Error("bundleId is empty");
  }

  const appAppleId = Number(request.appAppleId || 0) > 0 ? Number(request.appAppleId) : undefined;
  const verifier = new SignedDataVerifier(
    rootCertificates,
    Boolean(request.enableOnlineChecks),
    environment,
    bundleId,
    appAppleId,
  );

  if (signedPayload) {
    const notification = await verifier.verifyAndDecodeNotification(signedPayload);
    const signedTransactionInfo = String(notification?.data?.signedTransactionInfo || "").trim();
    const signedRenewalInfo = String(notification?.data?.signedRenewalInfo || "").trim();
    const transaction = signedTransactionInfo
      ? await verifier.verifyAndDecodeTransaction(signedTransactionInfo)
      : undefined;
    const renewalInfo = signedRenewalInfo
      ? await verifier.verifyAndDecodeRenewalInfo(signedRenewalInfo)
      : undefined;

    writeJSON({
      ok: true,
      notification,
      transaction,
      renewalInfo,
    });
  } else if (signedTransaction) {
    const transaction = await verifier.verifyAndDecodeTransaction(signedTransaction);
    writeJSON({
      ok: true,
      transaction,
    });
  } else {
    throw new Error("signedTransactionJWS or signedPayload is empty");
  }
} catch (error) {
  writeJSON({
    ok: false,
    error: error instanceof Error ? error.message : String(error),
  });
  process.exitCode = 1;
}
