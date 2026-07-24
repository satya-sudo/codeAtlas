# CodeAtlas MVP Plan

**Target release:** August 12, 2026
**Execution window:** July 27-August 12, 2026
**Product stage:** Polished end-to-end MVP

## Product Goal

CodeAtlas should help engineering teams understand the impact of a code change and involve the right people before it becomes a production problem.

The primary MVP journey is:

```text
Connect repository
  -> Build repository intelligence
  -> Open a change
  -> Understand risk and affected modules
  -> Select the right reviewers with evidence
```

The repository dashboard is a navigation and decision surface. It should prioritize actionable findings over disconnected repository statistics.

## Target User

The primary user is a tech lead, staff engineer, or engineering manager working in a team with:

- Several active repositories or a large monorepo
- Unclear ownership beyond `CODEOWNERS`
- Review bottlenecks around a few senior developers
- Developers joining, leaving, or moving between teams
- Legacy or unfamiliar modules that are risky to modify

Developers benefit directly from reviewer recommendations and change-impact analysis. Engineering leaders benefit from ownership, knowledge concentration, and architecture-risk visibility.

## MVP Scope

By August 12, a user must be able to:

- [ ] Sign in with GitHub.
- [ ] Install the CodeAtlas GitHub App.
- [ ] Connect and initially sync a repository.
- [ ] Receive incremental updates from GitHub webhooks.
- [ ] Open a focused repository dashboard.
- [ ] See prioritized repository risks and findings.
- [ ] Explore modules, hotspots, ownership, expertise, and co-change relationships.
- [ ] Open a pull request or recent change.
- [ ] See affected modules, risk level, and supporting evidence.
- [ ] Receive reviewer recommendations with explanations.
- [ ] Generate a feature-level, risk-ranked QA plan for a repository date range.
- [ ] Explore an observed dependency and knowledge graph.
- [ ] Ask supported, grounded repository questions through AI.

Detailed design: [Release QA Intelligence](./release-qa-intelligence.md)

## Product Principles

### Evidence Before Scores

Every risk score and reviewer recommendation must explain which repository facts produced it.

### Deterministic Analytics First

Ownership, expertise, hotspots, coupling, bus factor, change risk, and reviewer scoring are calculated by deterministic code. AI explains and connects verified results; it does not invent repository facts.

### Honest AI Provenance

CodeAtlas must not claim code is AI-generated based only on code style. It can report AI provenance when explicit metadata exists and can recommend enhanced review based on risk signals such as unfamiliar ownership, large changes, new dependencies, or missing tests.

### PostgreSQL And Neo4j Responsibilities

PostgreSQL remains the source of truth for users, repositories, sync state, imported GitHub data, and calculated metrics. Neo4j stores projected relationships used for traversal and impact analysis.

## Week 1: Intelligence Pipeline

### Day 1 - Monday, July 27: Product Contract

Backend:

- [ ] Define the pull-request domain model.
- [ ] Define the change-risk response format.
- [ ] Define the reviewer-recommendation response format.
- [ ] Define the repository-finding response format.
- [ ] Document sync and webhook status transitions.
- [ ] Confirm authoritative tables and endpoints.

Frontend:

- [ ] Freeze primary navigation: Repositories, Overview, Changes, Modules, Hotspots, Contributors, and Graph.
- [ ] Define loading, empty, error, retry, and stale-data states.
- [ ] Hide sections that do not support the primary journey.

**Exit criteria:** The end-to-end product flow and API contracts are documented and stable.

### Day 2 - Tuesday, July 28: Pull Request Ingestion

- [ ] Consume GitHub `pull_request` webhook deliveries.
- [ ] Publish normalized events to `github.pull_request`.
- [ ] Support `opened`, `synchronize`, `reopened`, and `closed`.
- [ ] Store PR ID, number, title, state, author, branches, SHAs, timestamps, changed files, additions, deletions, delivery ID, and analysis status.
- [ ] Make ingestion idempotent by GitHub PR identity and delivery ID.

**Exit criteria:** Opening or updating a PR creates or updates exactly one PR record.

### Day 3 - Wednesday, July 29: Change Analysis

- [ ] Calculate files and modules affected.
- [ ] Calculate total churn.
- [ ] Identify hotspots touched.
- [ ] Identify relevant co-change relationships.
- [ ] Measure author familiarity with affected modules.
- [ ] Measure ownership concentration.
- [ ] Detect whether tests changed with production code.
- [ ] Flag sensitive changes to authentication, migrations, infrastructure, and dependency manifests.
- [ ] Return `risk_level`, `risk_score`, `affected_modules`, and evidence-backed `findings`.

**Exit criteria:** Identical PR data always produces the same result, and every finding includes evidence.

### Day 4 - Thursday, July 30: Reviewer Recommendation

- [ ] Score file-level contribution history.
- [ ] Score module ownership and expertise.
- [ ] Include recent activity.
- [ ] Include familiarity with coupled files and modules.
- [ ] Exclude the change author.
- [ ] Avoid inactive contributors where possible.
- [ ] Return the candidate, score, known files/modules, and recommendation reasons.

**Exit criteria:** Every recommendation contains at least one concrete reason and never recommends the author.

### Day 5 - Friday, July 31: Repository Findings

Initial finding types:

- [ ] Bus factor of one.
- [ ] High ownership concentration.
- [ ] Hotspot without an active expert.
- [ ] High-churn module.
- [ ] Strong module coupling.
- [ ] Cross-module knowledge gap.
- [ ] Change touching an unfamiliar module.
- [ ] Change without corresponding tests.
- [ ] Failed or stale sync.

Endpoints:

```http
GET /repos/{id}/findings
GET /repos/{id}/findings/{finding_id}
```

**Exit criteria:** The dashboard receives prioritized findings from one API instead of reconstructing them from unrelated endpoints.

### Day 6 - Saturday, August 1: Graph Foundation

Nodes:

```text
Repository
Developer
Module
File
PullRequest
```

Relationships:

```text
Repository-[:HAS_MODULE]->Module
Module-[:CONTAINS]->File
Developer-[:CONTRIBUTED_TO]->File
Developer-[:EXPERT_IN]->Module
Developer-[:OWNS]->Module
File-[:CO_CHANGED_WITH]->File
Module-[:COUPLED_WITH]->Module
PullRequest-[:CHANGES]->File
```

- [ ] Add an idempotent graph projection after successful sync and PR analysis.
- [ ] Preserve PostgreSQL as the source of truth.
- [ ] Clearly label historical relationships as observed coupling.

**Exit criteria:** Replaying projection creates no duplicate nodes or relationships.

### Day 7 - Sunday, August 2: Stabilization Checkpoint

Validate:

- [ ] New GitHub App installation.
- [ ] Repository connection.
- [ ] Initial sync.
- [ ] Push webhook.
- [ ] Pull-request webhook.
- [ ] Duplicate delivery handling.
- [ ] Failed sync and retry.
- [ ] Graph projection.
- [ ] Service restart behavior.
- [ ] Empty repository.
- [ ] Repository with no contributors.
- [ ] Larger repository.

**Exit criteria:** Schema, transaction, and idempotency issues are fixed before frontend feature work expands.

## Week 2: Focused Product Experience

### Day 8 - Monday, August 3: Repository Home Redesign

Arrange the repository home in this order:

1. Needs attention
2. Recent changes
3. Knowledge health
4. Architecture and coupling
5. Repository statistics
6. Sync and integration status

- [ ] Make the highest-risk finding visible in the first viewport.
- [ ] Make findings actionable and link them to detail pages.
- [ ] Move general totals below decision-oriented content.

**Exit criteria:** A user can identify what needs attention without scanning unrelated metrics.

### Day 9 - Tuesday, August 4: Change Intelligence Page

- [ ] Show risk level and score.
- [ ] Show a concise change summary.
- [ ] Show affected modules.
- [ ] Show hotspots touched.
- [ ] Show potentially affected coupled files.
- [ ] Show reviewer recommendations and evidence.
- [ ] Show review warnings.
- [ ] Link back to the GitHub change.

**Exit criteria:** A tech lead can understand a change and choose reviewers from one page.

### Day 10 - Wednesday, August 5: Module Intelligence

Module list:

- [ ] Risk.
- [ ] Bus factor.
- [ ] Top owner.
- [ ] Reviewer depth.
- [ ] Churn.
- [ ] Recent activity.

Module detail:

- [ ] Ownership distribution.
- [ ] Experts and reviewer candidates.
- [ ] Hotspot files.
- [ ] Linked modules.
- [ ] Recent changes.
- [ ] Knowledge concentration.
- [ ] Files contained in the module.

**Exit criteria:** The list supports comparison, while the detail page supports investigation without duplicating the full list view.

### Day 11 - Thursday, August 6: Graph Experience

Focused graph views:

- [ ] Module dependency and coupling.
- [ ] Developer-to-module knowledge.
- [ ] Change impact for a selected PR.
- [ ] File neighborhood for a selected hotspot.

Interactions:

- [ ] Search.
- [ ] Filter by node type.
- [ ] Click for details.
- [ ] Expand one relationship level.
- [ ] Open module, file, contributor, or change detail.
- [ ] Show a clear relationship legend.

**Exit criteria:** Every graph view supports a specific investigation instead of displaying an unrestricted node cloud.

### Day 12 - Friday, August 7: Release QA Intelligence

- [ ] Accept a repository date range and resolve it against default-branch commits.
- [ ] Group changes by pull request, issue reference, or evidence-backed commit clustering.
- [ ] Label inferred groups with a confidence level.
- [ ] Map direct and potentially affected modules.
- [ ] Generate deterministic, risk-ranked QA recommendations.
- [ ] Show the contributing evidence for every priority.
- [ ] Keep analysis asynchronous for larger ranges.

**Exit criteria:** A user can request “Generate a QA plan from June 1 to June 12” and receive feature/change groups, impacted modules, risks, experts, and an evidence-backed QA checklist.

Detailed design: [Release QA Intelligence](./release-qa-intelligence.md)

### Grounded AI Stretch Scope

Supported questions:

- [ ] Who should review changes in this module?
- [ ] Why is this module high risk?
- [ ] What could be affected by changing this file?
- [ ] Where is repository knowledge concentrated?
- [ ] Which modules frequently change together?
- [ ] What should a new developer learn first?

Rules:

- [ ] Supply structured PostgreSQL and Neo4j facts to the model.
- [ ] Link answers to supporting files, modules, contributors, changes, and findings.
- [ ] Restrict answers to the selected repository.
- [ ] Provide deterministic fallback answers when AI configuration is unavailable.

**Exit criteria:** The assistant does not invent metrics and every material conclusion is grounded in repository evidence.

### Day 13 - Saturday, August 8: UX Polish

- [ ] Use consistent navigation, page titles, and breadcrumbs.
- [ ] Normalize typography, spacing, cards, buttons, and severity colors.
- [ ] Use middle ellipsis and tooltips for long paths.
- [ ] Verify laptop and mobile layouts.
- [ ] Add skeleton loading.
- [ ] Add clear empty, retryable error, and stale-data states.
- [ ] Clearly distinguish manual and webhook syncs.
- [ ] Add accessible keyboard focus.
- [ ] Explain technical terms such as bus factor and observed coupling.
- [ ] Remove repeated summaries, internal IDs as labels, distracting placeholders, and unexplained metrics.

### Day 14 - Sunday, August 9: End-To-End Testing

Test with:

- [ ] A small repository.
- [ ] A larger repository.
- [ ] An empty or low-activity repository.
- [ ] A PR touching a hotspot.
- [ ] A cross-module PR.
- [ ] A duplicate webhook delivery.
- [ ] A failed GitHub request.
- [ ] A retry after rate limiting.

Verify PostgreSQL, Kafka offsets, Neo4j projection, APIs, and frontend state together.

## Launch Buffer

### August 10: Feature Freeze

- [ ] Stop adding features.
- [ ] Fix only release-blocking bugs.
- [ ] Validate migrations from an empty database.
- [ ] Validate migrations against the current development database.
- [ ] Document environment variables.
- [ ] Verify Docker Compose startup.
- [ ] Verify GitHub App permissions and webhook setup.
- [ ] Run Go tests and builds.
- [ ] Run the frontend production build.

### August 11: Demo And Documentation

- [ ] Prepare a clean demo account and repository.
- [ ] Prepare a meaningful PR example.
- [ ] Update the architecture diagram.
- [ ] Complete the setup guide.
- [ ] Complete API documentation.
- [ ] Document how major scores are calculated.
- [ ] Document known limitations.
- [ ] Prepare a short product walkthrough.
- [ ] Run the complete demo twice from a fresh session.

### August 12: MVP Release

Release only when:

- [ ] Onboarding works without manual database intervention.
- [ ] Initial and incremental syncs work.
- [ ] Duplicate webhook deliveries are safe.
- [ ] The repository dashboard is focused.
- [ ] PR risk and reviewer recommendations use real data.
- [ ] Every risk score contains evidence.
- [ ] Graph views support specific investigation workflows.
- [ ] AI answers are grounded and optional.
- [ ] Empty, loading, failed, and stale states are polished.
- [ ] No fake production metrics appear anywhere.

## Priority And Cut Rules

If the schedule slips, cut in this order:

1. Free-form AI chat.
2. Advanced graph interaction.
3. Structural dependency parsing.
4. Additional finding types.
5. Secondary dashboard visualizations.

Do not cut:

- Reliable onboarding.
- Idempotent ingestion.
- Change analysis.
- Reviewer recommendations.
- Release QA analysis for a date range.
- Evidence behind scores.
- Focused repository dashboard.
- Error handling and polished states.

## Final Demo Story

The August 12 demonstration must follow one cohesive path:

```text
Sign in
  -> Connect a repository
  -> Complete the initial sync
  -> Receive a GitHub change
  -> Open change intelligence
  -> Understand risk and affected modules
  -> Review supporting evidence
  -> Select the right reviewers
  -> Explore the relevant knowledge graph
```
