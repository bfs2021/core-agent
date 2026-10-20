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
	"fmt"
	"strings"
)

// taskInput is everything the task text is built from. It holds the four
// issue fields decision 20 allows and nothing else: no labels (the
// should-escalate / should-stop expectation labels would hand the worker
// the answer key), no assignees, no comments. Building the text from this
// struct rather than from a ghIssue makes that a property of the type —
// there is no field to leak.
type taskInput struct {
	Number   int
	Title    string
	Body     string
	Upstream string

	Repo    string // owner/name of the mirror, for the reader's orientation
	Branch  string
	Dir     string // the per-issue working copy
	BaseSHA string
}

func branchFor(number int) string { return fmt.Sprintf("agent/issue-%d", number) }

// buildTask renders the inject. The issue's own words come first and
// verbatim; the rig's standing instructions follow.
//
// The working-copy instructions exist because POST /sessions cannot set a
// session's working directory: the session starts in the daemon's
// workspace, so the agent is told where its copy is and to address it with
// -C rather than cd (a compound `cd x && …` never matches an eligible
// prefix and would escalate every time).
//
// Every command shape this text names must be on the approver's eligible
// list in config.soak.json, as a single simple command. A shape that
// isn't goes to a person, and in A7's first run the first `go -C … test`
// waited out the 30-minute approval timeout with nobody there. The
// eligibility test in task_test.go holds the two files together.
func buildTask(in taskInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Issue #%d in %s: %s\n", in.Number, in.Repo, in.Title)
	fmt.Fprintf(&b, "Upstream issue: %s\n\n", in.Upstream)
	b.WriteString(strings.TrimSpace(in.Body))
	b.WriteString("\n\n---\n\n")
	fmt.Fprintf(&b, "Your working copy is %s, a fresh clone of the mirror's main at %s, with branch %s checked out.\n\n", in.Dir, in.BaseSHA, in.Branch)
	b.WriteString("How to work:\n")
	fmt.Fprintf(&b, "- Work only inside %s, and address the copy instead of changing directory: `go -C %s test ./pkg/...`, `git -C %s status`, and the repo's scripts by absolute path, such as `%s/dev/ci/presubmits/build`.\n", in.Dir, in.Dir, in.Dir, in.Dir)
	b.WriteString("- Make every command one simple command in exactly those forms: start it with `go -C`, `git -C` or the copy's path, unquoted, with no `VAR=value` prefix, and use no pipes, no redirections such as `2>&1`, no `&&` or `;`, and no `$VAR`, `$(...)` or backticks. The approver judges commands in those forms. Anything else goes to a person for approval and waits until someone answers or the request expires. Stdout and stderr come back as separate fields, so `2>&1` is not needed; to cut long test output, narrow the run with `-run` or a package path.\n")
	b.WriteString("- A command stops after 30 seconds unless you pass `timeout_seconds`; give test runs and presubmits enough time.\n")
	fmt.Fprintf(&b, "- Commit your work on %s with `git -C %s commit -s -F %s/.git/SOAK_COMMIT_MSG`, writing the message to that file first, using the git identity already configured in the copy.\n", in.Branch, in.Dir, in.Dir)
	b.WriteString("- Stop once the work is committed locally. Do not push, open a pull request, or call the GitHub API: the dispatcher pushes the branch and opens the pull request after your session goes idle.\n")
	fmt.Fprintf(&b, "- Cite the upstream issue (%s) in commit bodies and in any CHANGELOG bullet, so a cherry-pick upstream carries the right link.\n", in.Upstream)
	return b.String()
}
