# Self-development historical replay: re-implementing what we already merged

Status: proposed, 2026-10-19. Tracks [#1116](https://github.com/go-steer/core-agent/issues/1116); builds on [`selfdev-soak-design.md`](selfdev-soak-design.md) ([#1213](https://github.com/go-steer/core-agent/issues/1213)).

## Problem

Self-development tasks taken from the live issue queue go stale. Each one is a real unbuilt issue, and once a person builds it upstream, a run against current `main` has nothing to do. T1 retired that way when #1154 fixed #1002. T2 retired when #1162 shipped #954: a live T2 run on 2026-10-19 found the feature present, correctly built nothing, and failed every assertion that needs a change ([#1332](https://github.com/go-steer/core-agent/issues/1332)). T1 survived only because `--replay` pins it to a commit from before the fix.

A queue of live issues also can't be compared across agent builds, models or recipes, because the queue is different every time it is run.

History doesn't have either problem. Every merged fix or feature is a task with a known answer: the issue, the commit it started from, the merged diff, its tests, the review thread, and how long it took. Rewinding a repository to a past commit and handing the agent the issues that actually got from there to a later commit gives a fixed, repeatable task set. It also has a reference implementation for each task to judge against.

## Shape

One corpus, two ways to run it.

```
  corpus (outside the repo — the answer key)
  ├── task: base SHA · issue snapshot (problem only) · oracle · reference PR
  ├── task: …
  └── window: B..C  (an ordered run of tasks)

  replay mode (scored)                     rewind soak (endurance)
  each task from ITS OWN base SHA          one mirror rewound to B
  fresh clone, no history past base        seeds = the window's issues, in order
  agent → local commits → grade           agent PR → reviewer agent merges → next
  comparable across builds / models        the agent builds on its own earlier work
```

- **Replay mode** generalizes T1's `--replay`: a pinned base per task, a local mirror holding nothing past it, a snapshotted issue, and no GitHub credentials. It is the benchmark, and the score comes from here.
- **The rewind soak** is the A7 soak rig pointed at a rewound mirror. Its seed queue is a historical window's issues, served in merge order, with the reviewer agent merging as in the soak design. Divergence compounds: if the agent builds task 3 differently, tasks 4–10 land on a codebase the reference never had. That is the endurance question, and it is reported, not scored against the reference.

## Settled decisions (do not relitigate)

*Proposed. They become settled when the maintainer approves this doc.*

1. **Score in replay mode only.** Each task starts from its own reference parent, so one task's result never depends on another's. The rewind soak reports per-task outcomes and the final tree's health, but no task score against the reference after the first divergence.
2. **The corpus is the answer key and lives outside the repo,** under the same rule as the soak's seed list (soak decision 20): the rewritten issue text, the oracle tests, and the expected-diff summaries. This doc may name a window and pilot tasks, because the shadow repositories never contain history past their base, and upstream is out of the worker's reach (decision 4).
3. **No history past the base.** A replay clone is a local mirror cut at the task's base. The rewind soak's mirror is a **new** private repository whose `main` starts at B, not the existing soak mirror rewound: that one has force-push off by design (soak decision 2). Release tag names are placeholders pointing at no real commit, for tags that are ancestors of the base only (soak decision 20).
4. **The worker can't reach the answer.** As in T1 replay and the soak:
   - no `gh` credentials and no GitHub network;
   - no module proxy access to later core-agent tags: the Go module cache is pre-warmed for the base, with `GOFLAGS=-mod=mod` and `GOPROXY=off`;
   - a pinned agent build (soak decision 11).

   What can't be closed is the model's own training data: core-agent is public, so a model may have seen the merged code. The corpus favours recent merges, and every report states the caveat.
5. **Issues are rewritten to the problem, not the fix.** Our issues are often written after the investigation and name the fix, the file and the line. The snapshot keeps the symptom, the reproduction and the acceptance criteria, and drops the diagnosis. A task whose issue can't be stripped without losing the problem is left out. The original issue is kept in the corpus for the judge.
6. **Grade by behaviour first, similarity never.** Different correct implementations don't look alike, so diff similarity isn't a metric. In order of strength:
   - **(a) The oracle.** The reference PR's own tests, when they exercise behaviour through public surfaces; otherwise a T1-style oracle written for the task, which must fail at the base and pass at the reference. A test that pins internal names the agent couldn't have guessed is rewritten against the behaviour or dropped.
   - **(b) The presubmit sweep,** the same 16 scripts CI runs.
   - **(c) A rubric judge comparing against the reference,** on a different provider from the worker (soak decision 18). It scores scope, size relative to the reference, test quality, files touched outside the reference's footprint, and whether the CHANGELOG and docs moved with the change.
   - **(d) Human spot checks** on a sample, and on every task where (a) and (c) disagree.
7. **Task selection rules for the scored corpus.** The rewind soak takes a window's `feat`/`fix` merges, minus the drop list, regardless of size.
   - **Keep:** a merged `feat` or `fix` with a clear issue (or a PR description that can stand as one), at least one behavioural test, and under ~1,000 changed lines.
   - **Drop:** docs-only changes, release cuts, bot pin bumps, rig-only `dev/uat` changes that need a live cluster, and refactors whose task is the refactor itself (the ADK v2 port is not a fair task).
8. **Same pass/fail vocabulary as T0–T2.** A replay task's scorecard reuses the self-dev assertion names where they apply (A5 plan, A6 commits, A7 scope, A8 attribution, A13 tests, A14 CHANGELOG, A15 oracle). The new rows are judge scores and the reference comparison.

## The corpus entry

```yaml
id: r-1162
upstream_issue: 954            # kept for the judge, never shown to the worker
reference_pr: 1162
base: 51fba1bf^                # the reference's parent
issue_snapshot: tasks/r-1162.md   # problem-only rewrite (decision 5)
oracle:
  kind: reference-tests | written
  run: go test ./pkg/tools/ -run '^TestViewFileOutline'
  fails_at_base: true          # checked when the entry is added
  passes_at_reference: true
footprint: [pkg/tools/, docs/site/…/tools.md, CHANGELOG.md]   # for the judge
budget: { usd: 15, wallclock: 2h }
```

Adding an entry runs its oracle at the base (must fail) and at the reference (must pass), the same check T1's A15 does per run.

## Pilot (replay mode)

Five tasks, chosen to cover a feature, a small fix, a medium feature and the two retired tiers:

| task | reference | size | why |
|---|---|---|---|
| r-1162 | #1162 `view_file_outline` (#954) | +1009/−89, 4 test files | Retires #1332: T2 becomes a replay task. |
| r-1154 | #1154 keep a subagent's banked return (#1002) | +709/−37, 5 | T1 as it is today; the control. |
| r-1171 | #1171 `stop_agent` says when the subagent had already finished | +114/−9, 1 | A small, sharp fix. |
| r-1229 | #1229 the operator's compaction threshold applies to every recognised model (#1226) | +156/−27, 2 | Config precedence with a crisp behavioural test. |
| r-1174 | #1174 a deny can carry the operator's reason (#1165) | +433/−7, 3 | A cross-package feature: attach, permissions, agent. |

Run each on the current release build, Claude Sonnet 5 per soak decision 14, before anything else is built on top. The pilot exists to calibrate oracles, judge rubric and budgets.

## Rewind soak window (proposed)

B = `51fba1bf^`, the parent of #1162 (2026-09-30). C = `a8dd5559`, #1233 (2026-10-03). The window holds 37 commits, 28 of them `feat` or `fix`. Dropping rig-only `dev/uat` and eval-corpus changes leaves about 24 seeds:
- #1162 itself;
- the #1171–#1185 permission and attach fixes;
- the auto-mode train #1186–#1211;
- #1198, #1202, #1204, #1225, #1229 and #1233.

Decision 7's size cap applies to the **scored corpus**, not the rewind soak. The soak deliberately keeps the large auto-mode phases (#1186, #1188 and #1197 are each 1,400–1,900 lines), because five phases that build on each other are exactly what an endurance run is for.

## Phases

1. **This doc,** approved, with the decisions settled.
2. **The corpus tooling:**
   - an entry format and a validator, which checks fails-at-base and passes-at-reference;
   - the five pilot entries, kept outside the repo;
   - `run.sh --replay --task <id>` in `dev/uat/self-dev/`, generalizing T1's replay;
   - the judge prompt and the scorecard rows.
3. **The pilot:** run the five tasks, then adjust oracles, the rubric and budgets from the results.
4. **The rewind soak:** create the new mirror at B, seed the window, run it with the soak rig, and report.

## Risks

- **Training-data contamination** (decision 4). This is unavoidable and is reported. A model that reproduces the reference near-verbatim is a signal to look at, not a pass.
- **Oracle brittleness.** Reference tests that pin internals fail a correct alternative design. Decision 6(a) rewrites or drops them; the validator only checks direction (fails at base, passes at reference), not fairness. The judge and the human spot checks are the backstop.
- **Issue rewriting is judgement.** A rewrite that leaks the fix inflates scores, and one that loses the problem deflates them. Each rewrite is reviewed against the original before an entry is added.
- **Cost.** Each task runs about the size of a T1 run. The pilot's five tasks are bounded by the per-entry budget, and the rewind soak by the soak's daily cap.

## Out of scope

- Ranking models or publishing scores outside the project.
- Tasks from other repositories.
- Using the corpus as training data.
- Automatically opening PRs upstream from replay results. As in the soak (decisions 3 and 13), anything worth keeping goes upstream through the normal gate.
- Self-selection of tasks by the agent.
