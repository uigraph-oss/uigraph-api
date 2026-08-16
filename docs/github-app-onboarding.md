# GitHub App repository onboarding

UiGraph can use a cloud-managed or self-hosted GitHub App to onboard up to 25 repositories in one batch to one UiGraph team. GitHub installation access tokens are minted only for API calls and are never persisted. OAuth user tokens and plaintext UiGraph onboarding tokens are also never persisted.

## Environment

The integration is disabled by default. Enabling it requires every required value; partial configuration and configuration while disabled fail startup.

| Variable | Required | Description |
|---|---:|---|
| `GITHUB_APP_ENABLED` | Yes | Must be `true` when the remaining GitHub App variables are set. |
| `GITHUB_APP_ID` | Yes | Numeric GitHub App ID. |
| `GITHUB_APP_SLUG` | Yes | App slug used for the installation URL. |
| `GITHUB_APP_CLIENT_ID` | Yes | OAuth client ID for GitHub user authorization. |
| `GITHUB_APP_CLIENT_SECRET` | Yes | OAuth client secret. |
| `GITHUB_APP_PRIVATE_KEY_BASE64` | Yes | Base64-encoded PEM RSA private key for app JWT signing. |
| `GITHUB_WEBHOOK_SECRET` | No | High-entropy webhook HMAC secret. When omitted, onboarding progress is polled from GitHub. |
| `GITHUB_API_URL` | No | REST API root, including an enterprise API prefix when needed. Defaults to `https://api.github.com/`. |
| `GITHUB_WEB_URL` | No | Browser GitHub root. Defaults to `https://github.com`. |
| `UIGRAPH_PUBLIC_URL` | Yes in production | Externally reachable API base used for `/api/v1/github-app/callback`. |
| `UIGRAPH_FRONTEND_URL` | No | Post-install UI redirect base. Falls back to `UIGRAPH_PUBLIC_URL`. |

Example private-key conversion:

```bash
base64 < github-app.private-key.pem | tr -d '\n'
```

## GitHub App configuration

Repository permissions:

| Permission | Access |
|---|---|
| Metadata | Read |
| Contents | Read and write |
| Pull requests | Read and write |
| Workflows | Read and write |
| Actions | Read and write |
| Secrets | Read and write |
| Variables | Read |

Organization permissions:

| Permission | Access |
|---|---|
| Secrets | Read and write |
| Variables | Read |

When webhooks are enabled, subscribe to `installation`, `installation_repositories`, `pull_request`, `workflow_run`, `repository`, and `ping`. Set the webhook URL to `${UIGRAPH_PUBLIC_URL}/api/v1/github-app/webhooks` and use the exact `GITHUB_WEBHOOK_SECRET` value. Without a webhook secret, UI status polling reconciles pull requests and workflow runs directly from GitHub.

Set the GitHub App setup URL and OAuth callback URL to `${UIGRAPH_PUBLIC_URL}/api/v1/github-app/callback`. The API uses an expiring state and authorizes the GitHub user both before installation and after setup. Final setup succeeds only when that same GitHub user can access the installation and the installation belongs to the configured app.

Repositories must permit GitHub Actions to create pull requests. If organization or repository policy disables this setting, generation fails with an actionable onboarding error.

## Customer AI settings

After the setup pull request is merged, UiGraph waits until these Actions settings are visible at repository or organization scope:

| Name | Type | Required |
|---|---|---:|
| `AI_PROVIDER_API_KEY` | Secret | Yes |
| `AI_PROVIDER_MODEL` | Variable | Yes |
| `AI_PROVIDER_API_URL` | Variable | Yes |
| `AI_PROVIDER_NPM` | Variable | No |

`UIGRAPH_TOKEN` is not available to generation. The generated sync workflow maps the limited `UIGRAPH_ONBOARDING_TOKEN` Actions secret to `UIGRAPH_TOKEN` only for an explicitly dispatched sync run.

For organization installations, UiGraph creates `UIGRAPH_ONBOARDING_TOKEN` with `selected` visibility limited to repositories in the onboarding batch. Personal installations use one repository secret per selected repository.

## Orchestration

1. PR1 atomically adds `.uigraph.yaml`, `.github/workflows/uigraph-generate.yml`, and `.github/workflows/uigraph-sync.yml` on `uigraph/setup/{onboardingID}`.
2. After PR1 merge, the API reports missing AI setting names or dispatches generation.
3. Generation runs pinned agents with `artifacts init --seeded`, validates with a pinned CLI, restricts writes to `.uigraph.yaml` and `.uigraph/**`, and opens PR2 from `uigraph/generated/{onboardingID}`.
4. After PR2 merge, the API creates the limited, expiring service-account token, installs the Actions secret, and dispatches sync.
5. A successful sync links the onboarding to the service whose canonical Git repository URL matches and completes the batch item.

Closing either expected pull request without merging marks the item failed. Retry and AI recheck are explicit protected API operations. Durable Postgres jobs use leases, exponential retries, and bounded attempts so process restarts do not lose orchestration work.
