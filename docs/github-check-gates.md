# GitHub Check Gates

Agora DE runs the same repo gate locally and in GitHub Actions:

```bash
./harness/ci/check-all.sh
```

The workflow is `.github/workflows/offline-ci.yml`. It runs automatically on
pushes to `main`, pull requests, and manual dispatch.

For Den Review GitHub check gates, use:

```json
{
  "project_id": "agora-os",
  "task_id": "<den-task-id>",
  "repository": "FuzzySlipper/agora-de",
  "commit_sha": "<full-40-character-sha>",
  "ref": "main",
  "required_checks": ["Verify Agora DE"],
  "requested_by": "<agent-name>"
}
```

Agents should register the exact pushed commit SHA after a task commit is
pushed. The Den service records pass, fail, timeout, or superseded evidence on
the task thread; GitHub Actions remains the runner.

`Verify Agora DE` is the GitHub Actions job/check-run name from
`.github/workflows/offline-ci.yml`. Do not use the workflow file name as the
required check.
