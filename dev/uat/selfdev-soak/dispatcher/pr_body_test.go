// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"strings"
	"testing"
)

// The agent can't edit the PR body, and the repository's review-gate
// check fails a Go-touching PR whose body never says "adversarial
// review". The task text has the agent record its review in the commit
// body, so the body must carry the commit messages, and no cut may take
// the review.
func TestPRBody_CarriesCommitMessages(t *testing.T) {
	a := &activeIssue{Number: 2, Upstream: testUpstream, SessionID: "sess-1", BaseSHA: strings.Repeat("a", 40)}
	commits := []commit{ // newest first, as git log lists them
		{SHA: strings.Repeat("2", 40), Message: "docs: note the fallback\n\nFixes #9 by documenting it.\n"},
		{SHA: strings.Repeat("1", 40), Message: "fix(agent): fall back on the first empty summary\n\nAdversarial review: the reviewer found one gap; fixed.\n"},
	}
	body := prBody(a, "o/r", "1. Fix it.", commits)

	first := strings.Index(body, "fall back on the first empty summary")
	second := strings.Index(body, "docs: note the fallback")
	if first < 0 || second < 0 || first > second {
		t.Errorf("commit messages missing or not oldest first:\n%s", body)
	}
	if !strings.Contains(body, "Adversarial review:") {
		t.Errorf("the review in the commit body did not reach the PR body:\n%s", body)
	}
	// Only the mirror issue is closed by the PR; a closer in a commit
	// message must not close another.
	if strings.Contains(body, "Fixes #9") {
		t.Errorf("a commit message's closer survived:\n%s", body)
	}
}

// Review finding: a cut that kept the start dropped the review, which the
// task text puts at the end of the newest commit's body. Past the budget
// the cut takes the start, and with an empty plan the messages get the
// whole budget uncut.
func TestPRBody_CutKeepsTheReview(t *testing.T) {
	a := &activeIssue{Number: 2, Upstream: testUpstream, SessionID: "sess-1", BaseSHA: strings.Repeat("a", 40)}
	long := strings.Repeat("why ", 5000) // 20000 bytes, past minCommitsBytes
	commits := []commit{
		{SHA: strings.Repeat("2", 40), Message: "fix: y\n\nAdversarial review: one finding, fixed."},
		{SHA: strings.Repeat("1", 40), Message: "fix: x\n\n" + long},
	}
	full := prBody(a, "o/r", strings.Repeat("p", maxPlanBytes), commits)
	if !strings.Contains(full, "Adversarial review:") || len(full) >= 65536 {
		t.Errorf("full plan: review kept %v, %d bytes; want the review and under 65536", strings.Contains(full, "Adversarial review:"), len(full))
	}
	empty := prBody(a, "o/r", "", commits)
	if strings.Contains(empty, "truncated") || !strings.Contains(empty, strings.TrimSpace(long)) {
		t.Error("with no plan the commit messages were cut, though the budget had room")
	}
}

// GitHub refuses a body over 65536 characters. A plan at its own cap and
// long commit messages together must fit, and the plan is what gives.
func TestPRBody_FitsWithAFullPlan(t *testing.T) {
	a := &activeIssue{Number: 2, Upstream: testUpstream, SessionID: "sess-1", BaseSHA: strings.Repeat("a", 40)}
	review := "\n\nAdversarial review: one finding, fixed."
	commits := []commit{{SHA: strings.Repeat("1", 40), Message: "fix: x\n\n" + strings.Repeat("why ", 2000) + review}}
	body := prBody(a, "o/r", strings.Repeat("p", maxPlanBytes), commits)
	if len(body) >= 65536 {
		t.Errorf("body is %d bytes, want under GitHub's 65536-character limit", len(body))
	}
	if !strings.Contains(body, "Adversarial review:") {
		t.Error("the plan's size pushed the review out of the body")
	}
}
