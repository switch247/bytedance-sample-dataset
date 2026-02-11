#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const { spawnSync } = require("node:child_process");

const ROOT = path.resolve(__dirname, "..");
const TMP_DIR = path.join(ROOT, "evaluation", "tmp");
const runId = () => crypto.randomBytes(4).toString("hex");

const runJest = (label, scriptName, env) => {
  console.log(`\n${"=".repeat(60)}`);
  console.log(`RUNNING TESTS: ${label.toUpperCase()}`);
  console.log(`${"=".repeat(60)}`);

  const outputFileName = `results-${label}.json`;
  const relativePath = path.join("evaluation", "tmp", outputFileName);
  const absolutePath = path.join(ROOT, relativePath);
  
  if (!fs.existsSync(path.dirname(absolutePath))) {
    fs.mkdirSync(path.dirname(absolutePath), { recursive: true });
  }

  const cmd = process.platform === "win32" ? "npm.cmd" : "npm";
  const args = ["run", scriptName, "--", "--json", "--outputFile", relativePath, "--runInBand"];

  console.log(`Executing: ${cmd} ${args.join(" ")}`);

  const result = spawnSync(cmd, args, {
    cwd: ROOT,
    encoding: "utf8",
    env: { ...process.env, ...env },
    shell: true
  });

  let parsed = null;
  if (fs.existsSync(absolutePath)) {
    try {
      parsed = JSON.parse(fs.readFileSync(absolutePath, "utf8"));
    } catch (e) {
      console.error(`Failed to parse ${absolutePath}: ${e.message}`);
    }
  }

  const tests = [];
  if (parsed && Array.isArray(parsed.testResults)) {
    for (const suite of parsed.testResults) {
      for (const test of suite.assertionResults || []) {
        tests.push({
          name: test.title,
          outcome: test.status
        });
      }
    }
  }

  const summary = {
    total: tests.length,
    passed: tests.filter((t) => t.outcome === "passed" || t.outcome === "success").length,
    failed: tests.filter((t) => t.outcome === "failed" || t.outcome === "failure").length,
  };

  if (tests.length > 0) {
    console.log(`Results: ${summary.passed} passed, ${summary.failed} failed`);
    for (const test of tests) {
      const icon = test.outcome === "passed" ? "✅" : "❌";
      console.log(`  ${icon} ${test.name}: ${test.outcome}`);
    }
  } else {
    console.log("CRITICAL: No test results captured.");
    if (result.stderr && !parsed) console.error(result.stderr);
  }

  return {
    success: summary.total > 0 && summary.failed === 0,
    summary,
    tests
  };
};

const runEvaluation = () => {
  const startedAt = new Date();
  
  // Execute both suites
  const beforeResults = runJest("before", "test:before", { TEST_TARGET: "before" });
  const afterResults = runJest("after", "test:after", { TEST_TARGET: "after" });

  const report = {
    run_id: runId(),
    timestamp: startedAt.toISOString(),
    results: {
      before: beforeResults.summary,
      after: afterResults.summary
    },
    passed_evaluation: afterResults.summary.total > 0 && afterResults.summary.failed === 0
  };

  const dateStr = startedAt.toISOString().split("T")[0]; // YYYY-MM-DD
  const timeStr = startedAt.toISOString().split("T")[1].replace(/:/g, "-").split(".")[0]; // HH-MM-SS
  const reportDir = path.join(ROOT, "evaluation", "reports", dateStr, timeStr);
  if (!fs.existsSync(reportDir)) fs.mkdirSync(reportDir, { recursive: true });
  const reportPath = path.join(reportDir, "report.json");
  
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));

  console.log(`\n${"=".repeat(60)}`);
  console.log(`FINAL SUMMARY`);
  console.log(`${"=".repeat(60)}`);
  console.log(`Before (Legacy): ${report.results.before.passed}/${report.results.before.total} passed`);
  console.log(`After (Refactored): ${report.results.after.passed}/${report.results.after.total} passed`);
  console.log(`Report: ${reportPath}`);
  
  // Cleanup tmp directory
  if (fs.existsSync(TMP_DIR)) {
    try {
      fs.rmSync(TMP_DIR, { recursive: true, force: true });
    } catch (err) {
      console.error(`Warning: Failed to cleanup ${TMP_DIR}: ${err.message}`);
    }
  }

  if (report.passed_evaluation) {
    console.log("\n✅ EVALUATION SUCCESS: Fastify implementation meets all functional requirements.");
    process.exit(0);
  } else {
    console.log("\n❌ EVALUATION FAILED: Fastify implementation has failing tests.");
    process.exit(1);
  }
};

runEvaluation();
