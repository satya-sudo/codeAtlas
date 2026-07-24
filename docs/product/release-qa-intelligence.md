# Release QA Intelligence

**Status:** Proposed MVP feature
**Target release:** August 12, 2026
**Primary user:** Tech lead, QA lead, release manager, or engineer preparing a release

## User Problem

Before a release, teams often know which commits or pull requests were merged but still have to manually answer:

- What user-facing features changed?
- Which codebase modules were touched?
- Which related modules might also be affected?
- Where should QA spend the most time?
- Which changes are risky because of hotspots, weak ownership, or missing tests?
- Who can explain or review each area?

CodeAtlas should convert a repository change range into an evidence-backed release test plan.

## Primary User Request

```text
Generate a feature-level QA list for changes from June 1 to June 12.
```

The date range is an example. The user can select any valid range within the repository history available to CodeAtlas.

## Product Output

The result should contain:

1. A release summary.
2. Feature or change-group summaries.
3. Directly changed modules.
4. Potentially affected linked modules.
5. Risk-ranked QA focus areas.
6. Concrete test recommendations.
7. Suggested subject-matter experts.
8. Evidence for every conclusion.

Example:

```text
Release window: June 1-June 12
Changes analyzed: 18 pull requests, 42 commits, 96 files
QA priority: High

Feature: GitHub repository onboarding
Confidence: High

Directly changed:
- repo-service
- frontend

Potentially affected:
- auth-service, because installation claims depend on authenticated users
- webhook-service, because repository connection creates webhook configuration

Why this needs focused QA:
- Repository connection logic changed
- GitHub installation callback changed
- Two hotspot files were modified
- The author has limited history in webhook-service

Recommended QA:
- Install the GitHub App on a new account
- Connect a public repository
- Connect a private repository
- Repeat the callback and verify idempotency
- Verify webhook registration and signature validation
- Verify an unauthorized user cannot claim the installation

Suggested experts:
- satya-sudo: repository onboarding and GitHub App integration
```

## User Flow

```text
Repository dashboard
  -> Release QA
  -> Select a date range or Git reference range
  -> Generate analysis
  -> Review feature groups
  -> Inspect impacted modules and evidence
  -> Export or check off the QA plan
```

The first MVP supports a date range. A later version can support:

- Tag to tag
- Commit SHA to commit SHA
- Release branch to main
- GitHub release
- Deployment range

## Range Semantics

Date ranges can become ambiguous, so the UI and API must define them precisely.

MVP rule:

- Use commits on the repository default branch.
- Include commits where `committed_at >= from` and `committed_at < to + 1 day`.
- Interpret dates in the user's selected timezone.
- Display the resolved UTC timestamps in analysis metadata.
- Include merged pull requests when their merge commit is included in the range.

The result must show the exact resolved range so users can reproduce it.

## Feature-Level Mapping

CodeAtlas cannot safely assume every commit equals one product feature. It should build change groups using a confidence hierarchy.

### High Confidence

- One merged pull request and its commits
- Multiple commits linked to the same pull request
- Pull requests linked to the same issue or project item
- Explicit release or feature labels

### Medium Confidence

- Commits sharing an issue reference
- Commits with strongly similar messages
- Commits touching the same modules and files within a short time window
- Commits from the same branch metadata when available

### Low Confidence

- Clusters inferred only from file overlap, contributor, or timing
- Standalone commits with vague messages
- Automated dependency or formatting updates without useful metadata

Each generated feature group must include:

```text
id
title
summary
confidence
source_type
pull_request_ids
commit_shas
issue_references
changed_files
direct_modules
affected_modules
authors
```

Low-confidence groups must be labelled `Change group`, not asserted to be a product feature.

## AI Responsibility

AI may:

- Produce a readable title from PR titles and commit messages.
- Summarize a verified group of commits and files.
- Turn deterministic risk findings into a readable QA checklist.
- Merge duplicate test recommendations.
- Explain why a related module may need regression testing.

AI must not:

- Invent a feature not supported by commits, PRs, issues, or files.
- Claim a module is affected without a recorded dependency or coupling signal.
- Claim code was AI-generated without explicit provenance.
- Replace deterministic risk scoring.

Every AI-generated statement must be grounded in IDs and facts supplied by CodeAtlas.

## Module Impact Mapping

### Direct Impact

A module is directly impacted when one or more changed files belong to it.

Evidence:

```text
commit -> changed file -> belongs to module
pull request -> changed file -> belongs to module
```

### Expanded Impact

A module is potentially affected when it is connected to a directly changed module through:

- Structural imports or dependency references
- Observed module co-change
- Shared configuration
- Shared database migrations or schemas
- Shared API contracts
- Shared ownership with a high-risk direct module

Expanded impact must display the relationship type:

```text
Structural dependency
Observed coupling
Shared contract
Shared infrastructure
```

For the first MVP, CodeAtlas can use existing observed module co-change. It must not present observed coupling as a confirmed runtime dependency.

## QA Priority Score

The QA priority score should be deterministic and explainable.

Proposed inputs:

| Signal | Example Weight |
|---|---:|
| Sensitive authentication, migration, infrastructure, or dependency file changed | 20 |
| High-risk or hotspot file changed | 15 |
| Multiple modules directly changed | 12 |
| Strongly coupled module affected | 10 |
| Large churn | 10 |
| Author has low expertise in the module | 10 |
| Module bus factor is one | 8 |
| No relevant test files changed | 8 |
| Public API or shared contract changed | 7 |

The initial weights are product defaults and must be documented. We should later validate and tune them using real incidents and user feedback.

Suggested levels:

```text
0-24   Low
25-49  Medium
50-74  High
75-100 Critical
```

The response must include the contributing signals instead of returning only a number.

## QA Recommendation Types

Recommendations should be generated from known change patterns.

### Authentication And Authorization

- Login and callback flows
- Token validation
- Unauthorized access
- Cross-user data isolation
- Expired or invalid state

### Database And Migrations

- Migration from the previous schema
- Migration on an empty database
- Rollback or failure behavior where supported
- Duplicate and idempotent writes
- Existing-data compatibility

### API Contract

- Successful request
- Validation errors
- Authentication failures
- Backward compatibility
- Pagination and empty responses

### Event-Driven Processing

- Initial event
- Duplicate delivery
- Out-of-order event
- Consumer restart
- Failed processing and retry policy
- Idempotent database result

### Frontend

- Loading, empty, error, stale, and success states
- Navigation to affected feature pages
- Long names and large datasets
- Permissions and expired sessions
- Responsive layout

### Dependency And Infrastructure

- Service unavailable
- Rate limiting
- Timeout
- Partial failure
- Configuration validation
- Dependency version compatibility

## API Design

Analysis should run asynchronously because large date ranges may contain many commits and pull requests.

### Create Analysis

```http
POST /repos/{repositoryId}/release-analyses
Content-Type: application/json

{
  "range_type": "date",
  "from": "2026-06-01",
  "to": "2026-06-12",
  "timezone": "Asia/Kolkata"
}
```

Response:

```json
{
  "id": 42,
  "status": "queued"
}
```

### Get Analysis

```http
GET /repos/{repositoryId}/release-analyses/{analysisId}
```

### List Analyses

```http
GET /repos/{repositoryId}/release-analyses
```

### Re-run Analysis

```http
POST /repos/{repositoryId}/release-analyses/{analysisId}/retry
```

## Suggested Data Model

### `release_analyses`

```text
id
repository_id
requested_by_user_id
range_type
from_value
to_value
resolved_from_at
resolved_to_at
timezone
status
risk_score
risk_level
summary
error_message
created_at
started_at
completed_at
```

### `release_change_groups`

```text
id
release_analysis_id
title
summary
source_type
confidence
risk_score
risk_level
created_at
```

### `release_change_group_commits`

```text
release_change_group_id
commit_id
```

### `release_change_group_modules`

```text
release_change_group_id
module_id
impact_type
relationship_type
evidence
```

### `release_qa_items`

```text
id
release_analysis_id
change_group_id
module_id
priority
category
title
reason
evidence
status
```

## Service Ownership

| Responsibility | Service |
|---|---|
| Accept and authorize analysis requests | `repo-service` |
| Publish analysis request | `repo-service` |
| Build commit and PR range | `analytics-worker` |
| Calculate feature groups and QA risk | `analytics-worker` |
| Traverse module relationships | `graph-worker` or Neo4j-backed graph API |
| Persist analysis result | PostgreSQL through `analytics-worker` |
| Serve results | `repo-service` |
| Render analysis and checklist | `frontend` |
| Explain verified results | Optional AI layer |

Kafka topic:

```text
release.analysis.requested
```

Partition key:

```text
repository_id
```

## Frontend Design

The Release QA page should contain:

1. Range selector and generation action.
2. Analysis status and exact resolved range.
3. Release-level risk summary.
4. Feature/change-group list sorted by QA priority.
5. Direct and potentially affected modules.
6. Expandable evidence.
7. QA checklist grouped by feature and module.
8. Suggested experts.
9. Export action in a later iteration.

Avoid displaying every commit by default. Commits and files should be available as evidence inside an expanded feature group.

## Feature-Level QA Focus For CodeAtlas

This is the internal QA module map for implementing and releasing this capability.

| Priority | Module | Why It Needs Focus | Core QA |
|---|---|---|---|
| P0 | `sync-service` | Owns imported commit data and incremental correctness | Date boundaries, default branch, duplicate commits, missing history, large ranges |
| P0 | `repo-service` | Authorizes repository access and exposes analysis jobs | Cross-user access, invalid ranges, job idempotency, retry behavior |
| P0 | `analytics-worker` | Produces feature groups, impact, and QA priorities | Determinism, confidence labels, scoring evidence, empty ranges |
| P0 | PostgreSQL migrations and queries | Stores long-running analysis and evidence | Empty migration, existing data, uniqueness, transaction rollback |
| P1 | `webhook-service` | Supplies current PR and push facts | Duplicate delivery, signature validation, action normalization |
| P1 | Kafka | Connects request and analysis workers | Duplicate event, consumer restart, stale job, partition ordering |
| P1 | Module analytics | Expands direct changes into QA focus areas | Module derivation, root files, coupling thresholds, stale analytics |
| P1 | Frontend Release QA page | Converts analysis into an actionable workflow | Loading, polling, failure, retry, long content, evidence navigation |
| P2 | Neo4j and `graph-worker` | Enriches impact expansion | Projection idempotency, missing nodes, relationship labels |
| P2 | AI explanation | Improves summaries but must remain grounded | Unsupported claims, missing evidence, fallback without AI |

## MVP Acceptance Criteria

- [ ] A user can request an analysis for a valid date range.
- [ ] The analysis includes only default-branch commits in the resolved range.
- [ ] Pull requests are used as feature boundaries when available.
- [ ] Uncertain clusters are labelled as change groups with confidence.
- [ ] Directly changed modules are backed by changed-file evidence.
- [ ] Potentially affected modules identify the relationship type.
- [ ] QA priorities contain deterministic contributing signals.
- [ ] Every QA item explains why it was recommended.
- [ ] The result handles an empty date range cleanly.
- [ ] Duplicate analysis requests do not create conflicting active jobs.
- [ ] A user cannot analyze a repository they do not own.
- [ ] Large analyses run asynchronously and expose clear progress.
- [ ] AI is optional and cannot make the core result unavailable.

## MVP Cuts

To keep the August 12 release achievable:

- Support date ranges first.
- Use pull requests as the strongest feature boundary.
- Use existing module assignment and observed co-change.
- Generate test recommendations from deterministic templates.
- Use AI only for grounded summaries if time permits.
- Defer tag comparison, deployment comparison, Jira integration, export, and structural dependency parsing.
