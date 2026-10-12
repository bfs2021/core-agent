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

package attach_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cmgemini "github.com/go-steer/core-models/dialect/gemini"
	"github.com/go-steer/core-models/llm"
	"github.com/go-steer/core-models/retry"
	"google.golang.org/genai"

	"github.com/go-steer/core-agent/v3/pkg/attach"
)

// A model that sends no response headers within the provider library's
// header timeout fails with an error that wraps the transport's own
// context.Canceled. It must classify as a retryable transient failure,
// not as an operator cancel: a cancel is never re-driven and lets a
// pending guardrail take the blame for the cut. The error here is the
// real one, from core-models' Gemini client and retry transport, for a
// non-streaming call (the compaction summarizer's shape) and a
// streaming one.
func TestClassifyTurnError_HeaderTimeoutIsTransientNotCanceled(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // never send headers
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer srv.Close()

	pol := retry.Default()
	pol.MaxRetries = 0
	pol.HeaderTimeout = 200 * time.Millisecond      // streaming
	pol.UnaryHeaderTimeout = 200 * time.Millisecond // non-streaming (core-models v0.6.4+)
	c, err := cmgemini.New(context.Background(), cmgemini.Options{
		Backend: genai.BackendGeminiAPI, APIKey: "test", BaseURL: srv.URL, Retry: &pol,
	})
	if err != nil {
		t.Fatalf("cmgemini.New: %v", err)
	}

	for _, stream := range []bool{false, true} {
		req := &llm.Request{Contents: []*genai.Content{genai.NewContentFromText("hi", genai.RoleUser)}}
		var got error
		for _, e := range c.Model("gemini-3-pro").GenerateContent(context.Background(), req, stream) {
			if e != nil {
				got = fmt.Errorf("model call: %w", e)
			}
		}
		if got == nil {
			t.Fatalf("stream=%v: the call did not fail; the fixture never sends headers", stream)
		}
		if !errors.Is(got, retry.ErrNoResponse) {
			t.Fatalf("stream=%v: %v is not retry.ErrNoResponse; the fixture no longer exercises the header timeout", stream, got)
		}
		te := attach.ClassifyTurnError(got)
		if te.Kind != attach.TurnErrorTransientNet || !te.Retryable {
			t.Errorf("stream=%v: %v classified as kind=%s retryable=%v, want %s and retryable",
				stream, got, te.Kind, te.Retryable, attach.TurnErrorTransientNet)
		}
	}
}
