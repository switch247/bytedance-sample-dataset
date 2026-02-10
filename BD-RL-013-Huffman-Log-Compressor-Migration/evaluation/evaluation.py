#!/usr/bin/env python3
"""
Evaluation runner for Mechanical Refactor (calc_score).

This evaluation script:
- Runs pytest tests on the tests/ folder for both before and after implementations
- Collects individual test results with pass/fail status
- Generates structured reports with environment metadata

Run with:
    docker compose run --rm app python evaluation/evaluation.py [options]
"""
import os
import sys
import json
import uuid
import platform
import subprocess
import shutil
from datetime import datetime
from pathlib import Path


def generate_run_id():
    """Generate a short unique run ID."""
    return uuid.uuid4().hex[:8]


def get_git_info():
    """Get git commit and branch information."""
    git_info = {"git_commit": "unknown", "git_branch": "unknown"}
    try:
        result = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            git_info["git_commit"] = result.stdout.strip()[:8]
    except Exception:
        pass
    
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--abbrev-ref", "HEAD"],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            git_info["git_branch"] = result.stdout.strip()
    except Exception:
        pass
    
    return git_info


def get_environment_info():
    """Collect environment information for the report."""
    git_info = get_git_info()
    
    return {
        "python_version": platform.python_version(),
        "platform": platform.platform(),
        "os": platform.system(),
        "os_release": platform.release(),
        "architecture": platform.machine(),
        "hostname": platform.node(),
        "git_commit": git_info["git_commit"],
        "git_branch": git_info["git_branch"],
    }


def run_pytest_with_pythonpath(pythonpath, tests_dir, label):
    """
    Run pytest on the tests/ folder with specific PYTHONPATH.
    
    Args:
        pythonpath: The PYTHONPATH to use for the tests
        tests_dir: Path to the tests directory
        label: Label for this test run (e.g., "before", "after")
    
    Returns:
        dict with test results
    """
    print(f"\n{'=' * 60}")
    print(f"RUNNING TESTS: {label.upper()}")
    print(f"{'=' * 60}")
    print(f"PYTHONPATH: {pythonpath}")
    print(f"Tests directory: {tests_dir}")
    
    # Build pytest command
    cmd = [
        sys.executable, "-m", "pytest",
        str(tests_dir),
        "-v",
        "--tb=short",
    ]
    
    env = os.environ.copy()
    env["PYTHONPATH"] = pythonpath
    
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            cwd=str(Path(tests_dir).parent),
            env=env,
            timeout=120
        )
        
        stdout = result.stdout
        stderr = result.stderr
        
        # Parse verbose output to get test results
        tests = parse_pytest_verbose_output(stdout)
        
        # Count results
        passed = sum(1 for t in tests if t.get("outcome") == "passed")
        failed = sum(1 for t in tests if t.get("outcome") == "failed")
        errors = sum(1 for t in tests if t.get("outcome") == "error")
        skipped = sum(1 for t in tests if t.get("outcome") == "skipped")
        total = len(tests)
        
        print(f"\nResults: {passed} passed, {failed} failed, {errors} errors, {skipped} skipped (total: {total})")
        
        # Print individual test results
        for test in tests:
            status_icon = {
                "passed": "✅",
                "failed": "❌",
                "error": "💥",
                "skipped": "⏭️"
            }.get(test.get("outcome"), "❓")
            print(f"  {status_icon} {test.get('nodeid', 'unknown')}: {test.get('outcome', 'unknown')}")
        
        return {
            "success": result.returncode == 0,
            "exit_code": result.returncode,
            "tests": tests,
            "summary": {
                "total": total,
                "passed": passed,
                "failed": failed,
                "errors": errors,
                "skipped": skipped,
            },
            "stdout": stdout[-3000:] if len(stdout) > 3000 else stdout,
            "stderr": stderr[-1000:] if len(stderr) > 1000 else stderr,
        }
        
    except subprocess.TimeoutExpired:
        print("❌ Test execution timed out")
        return {
            "success": False,
            "exit_code": -1,
            "tests": [],
            "summary": {"error": "Test execution timed out"},
            "stdout": "",
            "stderr": "",
        }
    except Exception as e:
        print(f"❌ Error running tests: {e}")
        return {
            "success": False,
            "exit_code": -1,
            "tests": [],
            "summary": {"error": str(e)},
            "stdout": "",
            "stderr": "",
        }


def parse_pytest_verbose_output(output):
    """Parse pytest verbose output to extract test results."""
    tests = []
    lines = output.split('\n')
    
    for line in lines:
        line_stripped = line.strip()
        
        # Match lines like: tests/test_before.py::test_before_matches_reference_vectors PASSED
        if '::' in line_stripped:
            outcome = None
            if ' PASSED' in line_stripped:
                outcome = "passed"
            elif ' FAILED' in line_stripped:
                outcome = "failed"
            elif ' ERROR' in line_stripped:
                outcome = "error"
            elif ' SKIPPED' in line_stripped:
                outcome = "skipped"
            
            if outcome:
                # Extract nodeid (everything before the status)
                for status_word in [' PASSED', ' FAILED', ' ERROR', ' SKIPPED']:
                    if status_word in line_stripped:
                        nodeid = line_stripped.split(status_word)[0].strip()
                        break
                
                tests.append({
                    "nodeid": nodeid,
                    "name": nodeid.split("::")[-1] if "::" in nodeid else nodeid,
                    "outcome": outcome,
                })
    
    return tests


def parse_go_test_output(output):
    """Parse `go test -v` output for pass/fail/skip counts."""
    tests = []
    lines = output.splitlines()
    for line in lines:
        line = line.strip()
        if line.startswith('--- PASS:'):
            # format: --- PASS: TestName (0.00s)
            parts = line.split()
            if len(parts) >= 3:
                name = parts[2]
            else:
                name = line
            tests.append({"nodeid": name, "name": name, "outcome": "passed"})
        elif line.startswith('--- FAIL:'):
            parts = line.split()
            name = parts[2] if len(parts) >= 3 else line
            tests.append({"nodeid": name, "name": name, "outcome": "failed"})
        elif line.startswith('--- SKIP:'):
            parts = line.split()
            name = parts[2] if len(parts) >= 3 else line
            tests.append({"nodeid": name, "name": name, "outcome": "skipped"})
    return tests


def run_compose_service(service_name, label=None, timeout=600):
    """Run `docker compose run --rm --entrypoint "" <service_name>` and capture results.

    The service should have a `command:` configured in compose to execute tests.
    """
    if label is None:
        label = service_name

    print(f"\n{'=' * 60}")
    print(f"RUNNING DOCKER SERVICE: {label}")
    print(f"{'=' * 60}")

    docker = shutil.which("docker")
    if docker is None:
        return {
            "success": False,
            "exit_code": -1,
            "tests": [],
            "summary": {"error": "docker binary not found"},
            "stdout": "",
            "stderr": "",
        }

    cmd = [
        "docker",
        "compose",
        "run",
        "--rm",
        "--entrypoint",
        "",
        service_name,
    ]

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            cwd=str(Path(__file__).parent.parent),
            timeout=timeout,
        )

        stdout = result.stdout
        stderr = result.stderr

        # Try to detect whether this output is pytest or go test
        tests = []
        if 'pytest' in stdout or 'pytest' in stderr or 'FAILED' in stdout and 'PASSED' in stdout:
            # parse as pytest verbose output
            tests = parse_pytest_verbose_output(stdout)
        else:
            # attempt to parse as go test output
            tests = parse_go_test_output(stdout)

        passed = sum(1 for t in tests if t.get('outcome') == 'passed')
        failed = sum(1 for t in tests if t.get('outcome') == 'failed')
        skipped = sum(1 for t in tests if t.get('outcome') == 'skipped')
        total = len(tests)

        return {
            "success": result.returncode == 0,
            "exit_code": result.returncode,
            "tests": tests,
            "summary": {
                "total": total,
                "passed": passed,
                "failed": failed,
                "skipped": skipped,
            },
            "stdout": stdout[-3000:] if len(stdout) > 3000 else stdout,
            "stderr": stderr[-1000:] if len(stderr) > 1000 else stderr,
        }

    except subprocess.TimeoutExpired:
        return {
            "success": False,
            "exit_code": -1,
            "tests": [],
            "summary": {"error": "docker run timed out"},
            "stdout": "",
            "stderr": "",
        }
    except Exception as e:
        return {
            "success": False,
            "exit_code": -1,
            "tests": [],
            "summary": {"error": str(e)},
            "stdout": "",
            "stderr": "",
        }


def run_evaluation():
    """
    Run complete evaluation for both implementations.
    
    Returns dict with test results from both before and after implementations.
    """
    print(f"\n{'=' * 60}")
    print("MECHANICAL REFACTOR EVALUATION")
    print(f"{'=' * 60}")
    
    # Run the exact docker-compose commands provided by the project.
    # This function will execute these two commands and parse their outputs.
    project_root = Path(__file__).parent.parent

    def run_docker_cmd(cmd_list, label, timeout=900):
        print(f"\n{'=' * 60}")
        print(f"RUNNING: {label}")
        print(f"{'=' * 60}")
        print(' '.join(cmd_list))
        try:
            result = subprocess.run(
                cmd_list,
                capture_output=True,
                text=True,
                cwd=str(project_root),
                timeout=timeout,
            )
            stdout = result.stdout
            stderr = result.stderr

            # Choose parser based on label
            if 'pytest' in ' '.join(cmd_list) or 'before' in label.lower():
                tests = parse_pytest_verbose_output(stdout)
            else:
                tests = parse_go_test_output(stdout)

            passed = sum(1 for t in tests if t.get('outcome') == 'passed')
            failed = sum(1 for t in tests if t.get('outcome') == 'failed')
            skipped = sum(1 for t in tests if t.get('outcome') == 'skipped')
            total = len(tests)

            return {
                'success': result.returncode == 0,
                'exit_code': result.returncode,
                'tests': tests,
                'summary': {'total': total, 'passed': passed, 'failed': failed, 'skipped': skipped},
                'stdout': stdout[-3000:] if len(stdout) > 3000 else stdout,
                'stderr': stderr[-1000:] if len(stderr) > 1000 else stderr,
            }
        except subprocess.TimeoutExpired:
            return {'success': False, 'exit_code': -1, 'tests': [], 'summary': {'error': 'timeout'}, 'stdout': '', 'stderr': ''}

    # Exact commands (as requested):
    before_cmd = [
        'python3', '-m', 'pytest', '-v'
    ]

    after_cmd = [
        'go', 'test', './...', '-v'
    ]

    before_results = run_docker_cmd(before_cmd, 'before')
    after_results = run_docker_cmd(after_cmd, 'after')
    
    # Build comparison
    comparison = {
        "before_tests_passed": before_results.get("success", False),
        "after_tests_passed": after_results.get("success", False),
        "before_total": before_results.get("summary", {}).get("total", 0),
        "before_passed": before_results.get("summary", {}).get("passed", 0),
        "before_failed": before_results.get("summary", {}).get("failed", 0),
        "after_total": after_results.get("summary", {}).get("total", 0),
        "after_passed": after_results.get("summary", {}).get("passed", 0),
        "after_failed": after_results.get("summary", {}).get("failed", 0),
    }
    
    # Print summary
    print(f"\n{'=' * 60}")
    print("EVALUATION SUMMARY")
    print(f"{'=' * 60}")
    
    print(f"\nBefore Implementation (repository_before):")
    print(f"  Overall: {'✅ PASSED' if before_results.get('success') else '❌ FAILED'}")
    print(f"  Tests: {comparison['before_passed']}/{comparison['before_total']} passed")
    
    print(f"\nAfter Implementation (repository_after):")
    print(f"  Overall: {'✅ PASSED' if after_results.get('success') else '❌ FAILED'}")
    print(f"  Tests: {comparison['after_passed']}/{comparison['after_total']} passed")
    
    # Determine expected behavior
    print(f"\n{'=' * 60}")
    print("EXPECTED BEHAVIOR CHECK")
    print(f"{'=' * 60}")
    
    # Before: functional tests should pass, structural tests might fail
    # After: all tests should pass
    if after_results.get("success"):
        print("✅ After implementation: All tests passed (expected)")
    else:
        print("❌ After implementation: Some tests failed (unexpected - should pass all)")
    
    return {
        "before": before_results,
        "after": after_results,
        "comparison": comparison,
    }


def generate_output_path():
    """Generate output path in format: evaluation/YYYY-MM-DD/HH-MM-SS/report.json"""
    now = datetime.now()
    date_str = now.strftime("%Y-%m-%d")
    time_str = now.strftime("%H-%M-%S")
    
    project_root = Path(__file__).parent.parent
    output_dir = project_root / "evaluation" / date_str / time_str
    output_dir.mkdir(parents=True, exist_ok=True)
    
    return output_dir / "report.json"


def main():
    """Main entry point for evaluation."""
    import argparse
    
    parser = argparse.ArgumentParser(description="Run mechanical refactor evaluation")
    parser.add_argument(
        "--output", 
        type=str, 
        default=None, 
        help="Output JSON file path (default: evaluation/YYYY-MM-DD/HH-MM-SS/report.json)"
    )
    
    args = parser.parse_args()
    
    # Generate run ID and timestamps
    run_id = generate_run_id()
    started_at = datetime.now()
    
    print(f"Run ID: {run_id}")
    print(f"Started at: {started_at.isoformat()}")
    
    try:
        results = run_evaluation()
        
        # Success if after implementation passes all tests
        success = results["after"].get("success", False)
        error_message = None if success else "After implementation tests failed"
        
    except Exception as e:
        import traceback
        print(f"\nERROR: {str(e)}")
        traceback.print_exc()
        results = None
        success = False
        error_message = str(e)
    
    finished_at = datetime.now()
    duration = (finished_at - started_at).total_seconds()
    
    # Collect environment information
    environment = get_environment_info()
    
    # Build report
    report = {
        "run_id": run_id,
        "started_at": started_at.isoformat(),
        "finished_at": finished_at.isoformat(),
        "duration_seconds": round(duration, 6),
        "success": success,
        "error": error_message,
        "environment": environment,
        "results": results,
    }
    
    # Determine output path
    if args.output:
        output_path = Path(args.output)
    else:
        output_path = generate_output_path()
    
    output_path.parent.mkdir(parents=True, exist_ok=True)
    
    with open(output_path, "w") as f:
        json.dump(report, f, indent=2)
    print(f"\n✅ Report saved to: {output_path}")
    
    print(f"\n{'=' * 60}")
    print(f"EVALUATION COMPLETE")
    print(f"{'=' * 60}")
    print(f"Run ID: {run_id}")
    print(f"Duration: {duration:.2f}s")
    print(f"Success: {'✅ YES' if success else '❌ NO'}")
    
    return 0 if success else 1


if __name__ == "__main__":
    sys.exit(main())
