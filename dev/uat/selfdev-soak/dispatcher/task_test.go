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
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-steer/core-agent/v3/pkg/permissions"
)

// soakEligible builds the approver's eligible list exactly as the daemon
// does at startup: config.soak.json's patterns plus its bundles.
func soakEligible(t *testing.T) *permissions.Policy {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "config.soak.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Permissions struct {
			Auto struct {
				Eligible        []string `json:"eligible"`
				EligibleBundles []string `json:"eligible_bundles"`
			} `json:"auto"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	pats, err := permissions.ResolveAutoEligible(cfg.Permissions.Auto.Eligible, cfg.Permissions.Auto.EligibleBundles)
	if err != nil {
		t.Fatal(err)
	}
	pol, err := permissions.NewPolicy(pats, nil)
	if err != nil {
		t.Fatal(err)
	}
	return pol
}

// soakWorktreesDir reads --worktrees-dir from both dispatcher manifests,
// which must agree: it is where every per-issue copy lives.
func soakWorktreesDir(t *testing.T) string {
	t.Helper()
	flag := regexp.MustCompile(`"--worktrees-dir=([^"]+)"`)
	var dir string
	for _, f := range []string{"../deploy/base/51-deployment-dispatcher.yaml", "../deploy/a7-job/job-dispatcher-a7.yaml"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		m := flag.FindSubmatch(raw)
		if m == nil {
			t.Fatalf("%s sets no --worktrees-dir", f)
		}
		if dir != "" && string(m[1]) != dir {
			t.Fatalf("%s: --worktrees-dir=%s, but another manifest says %s", f, m[1], dir)
		}
		dir = string(m[1])
	}
	return dir
}

// In A7's first run the task text told the agent to run `go -C <copy> …`,
// and nothing on the eligible list could match that shape: the coding
// bundle's `go test *` needs the command to start with `go test`. The
// first test run went to a person, nobody answered, and the session ended
// thirty minutes later with the fix written and nothing committed. Every
// command the text names has to reach the approver as written.
func TestTaskCommands_ReachTheApprover(t *testing.T) {
	pol := soakEligible(t)
	dir := filepath.Join(soakWorktreesDir(t), "issue-7")
	text := buildTask(taskInput{
		Number: 7, Title: "t", Body: "b",
		Upstream: "https://github.com/go-steer/core-agent/issues/1",
		Repo:     "o/r", Branch: "agent/issue-7", Dir: dir,
		BaseSHA: strings.Repeat("a", 40),
	})
	_, rules, ok := strings.Cut(text, "How to work:")
	if !ok {
		t.Fatal("task text has no \"How to work:\" section")
	}

	// Every command the text names addresses the copy, so any backticked
	// span holding its path is one, whatever it starts with.
	var cmds []string
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(rules, -1) {
		if strings.Contains(m[1], dir) {
			cmds = append(cmds, m[1])
		}
	}
	// A test, a git read, a repo script and the commit. Fewer means the
	// text changed shape and this test stopped reading it.
	if len(cmds) < 4 {
		t.Fatalf("found %d commands in the task text, want at least 4: %q", len(cmds), cmds)
	}
	for _, c := range cmds {
		if pol.Match("bash", c) != permissions.OutcomeAllow {
			t.Errorf("the task text names %q, which is not on the approver's eligible list: the approver would not judge it", c)
		}
	}

	// The text asks for the recipe's reviewer subagent. In A7's second
	// run spawning it was not eligible, and the review never happened.
	if !strings.Contains(rules, "`reviewer` tool") {
		t.Error("the task text no longer asks for the reviewer; update this check with it")
	}
	if pol.Match("spawn_agent", "reviewer") != permissions.OutcomeAllow {
		t.Error("the task text asks for the reviewer, but calling it is not approver-eligible")
	}

	// The commit message file the text names is written with write_file.
	if msg := filepath.Join(dir, ".git", "SOAK_COMMIT_MSG"); pol.Match("write_file", msg) != permissions.OutcomeAllow {
		t.Errorf("write_file %s is not approver-eligible", msg)
	}

	// The text says the approver does not judge these shapes. If one became eligible,
	// the text would be wrong, and the design's guard with it.
	test := "go -C " + dir + " test ./pkg/agent/"
	for _, c := range []string{
		test + " 2>&1",
		test + " | tail -30",
		"cd " + dir + " && go test ./pkg/agent/",
		"git -C " + dir + " commit -s -m \"fix `x`\"",
		"GOFLAGS=-count=1 " + test,
		"go -C '" + dir + "' test ./pkg/agent/",
	} {
		if pol.Match("bash", c) == permissions.OutcomeAllow {
			t.Errorf("%q is approver-eligible, but the task text says the approver does not judge it", c)
		}
	}
}
