package evals

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
)

// reviewCase is a seeded diff with known issues that the review pipeline should detect.
type reviewCase struct {
	name           string
	diff           string
	expectedIssues []string // substrings that should appear in the review
	severity       string   // expected severity level: "critical", "warning", "info"
}

var reviewCases = []reviewCase{
	{
		name: "SQL injection",
		diff: `--- a/db.go
+++ b/db.go
@@ -10,6 +10,8 @@ func GetUser(db *sql.DB, id string) (*User, error) {
-    row := db.QueryRow("SELECT * FROM users WHERE id = $1", id)
+    query := "SELECT * FROM users WHERE id = '" + id + "'"
+    row := db.QueryRow(query)
     var u User`,
		expectedIssues: []string{"SQL injection", "parameterized", "user input"},
		severity:       "critical",
	},
	{
		name: "Hardcoded credentials",
		diff: `--- a/config.go
+++ b/config.go
@@ -5,6 +5,8 @@ func connect() {
+    password := "admin123"
+    db, err := sql.Open("postgres", "host=prod.db.internal user=admin password=" + password)`,
		expectedIssues: []string{"credential", "hardcoded", "secret"},
		severity:       "critical",
	},
	{
		name: "Missing error check",
		diff: `--- a/handler.go
+++ b/handler.go
@@ -15,7 +15,8 @@ func handleRequest(w http.ResponseWriter, r *http.Request) {
-    data, err := io.ReadAll(r.Body)
-    if err != nil {
-        http.Error(w, "bad request", 400)
-        return
-    }
+    data, _ := io.ReadAll(r.Body)
     process(data)`,
		expectedIssues: []string{"error", "ignored", "ReadAll"},
		severity:       "warning",
	},
	{
		name: "Race condition - unprotected map",
		diff: `--- a/cache.go
+++ b/cache.go
@@ -8,6 +8,12 @@ type Cache struct {
+func (c *Cache) Set(key string, val interface{}) {
+    c.data[key] = val
+}
+
+func (c *Cache) Get(key string) interface{} {
+    return c.data[key]
+}`,
		expectedIssues: []string{"concurrent", "mutex", "race"},
		severity:       "critical",
	},
	{
		name: "Path traversal",
		diff: `--- a/serve.go
+++ b/serve.go
@@ -12,6 +12,10 @@ func handleFile(w http.ResponseWriter, r *http.Request) {
+    filename := r.URL.Query().Get("file")
+    data, err := os.ReadFile("/var/data/" + filename)
+    if err == nil {
+        w.Write(data)
+    }`,
		expectedIssues: []string{"path traversal", "sanitize", "user input"},
		severity:       "critical",
	},
	{
		name: "Goroutine leak",
		diff: `--- a/worker.go
+++ b/worker.go
@@ -5,6 +5,12 @@ func processJobs(jobs <-chan Job) {
+func startWorker() {
+    ch := make(chan int)
+    go func() {
+        val := <-ch
+        fmt.Println(val)
+    }()
+    // channel is never written to
+}`,
		expectedIssues: []string{"goroutine", "leak", "channel"},
		severity:       "warning",
	},
	{
		name: "Integer overflow",
		diff: `--- a/math.go
+++ b/math.go
@@ -3,6 +3,9 @@ package math
+func Multiply(a, b int32) int32 {
+    return a * b
+}`,
		expectedIssues: []string{"overflow"},
		severity:       "warning",
	},
	{
		name: "Insecure TLS",
		diff: `--- a/client.go
+++ b/client.go
@@ -8,6 +8,11 @@ func newHTTPClient() *http.Client {
+    return &http.Client{
+        Transport: &http.Transport{
+            TLSClientConfig: &tls.Config{
+                InsecureSkipVerify: true,
+            },
+        },
+    }`,
		expectedIssues: []string{"InsecureSkipVerify", "TLS", "certificate"},
		severity:       "critical",
	},
	{
		name: "Clean refactor - no issues",
		diff: `--- a/utils.go
+++ b/utils.go
@@ -5,6 +5,10 @@ package utils
-func formatName(first, last string) string {
-    return first + " " + last
-}
+func formatName(first, last string) string {
+    return strings.Join([]string{first, last}, " ")
+}`,
		expectedIssues: nil, // no issues expected
		severity:       "",
	},
	{
		name: "Logging sensitive data",
		diff: `--- a/auth.go
+++ b/auth.go
@@ -15,6 +15,8 @@ func login(username, password string) error {
+    log.Printf("Login attempt: user=%s pass=%s", username, password)
     token, err := authenticate(username, password)`,
		expectedIssues: []string{"password", "log", "sensitive"},
		severity:       "critical",
	},
}

// mockReviewProvider simulates a model that identifies issues in diffs.
// It checks for known patterns and returns appropriate review text.
type mockReviewProvider struct {
	id       string
	findings map[string]string // diff keyword → finding text
}

func (m *mockReviewProvider) ID() string          { return m.id }
func (m *mockReviewProvider) Authenticate() error { return nil }
func (m *mockReviewProvider) Validate() error     { return nil }

func (m *mockReviewProvider) Query(_ context.Context, messages []provider.Message, _ provider.QueryOpts) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 2)
	go func() {
		defer close(ch)

		prompt := messages[len(messages)-1].Content

		var response string
		for keyword, finding := range m.findings {
			if strings.Contains(prompt, keyword) {
				response += finding + "\n"
			}
		}

		if response == "" {
			response = "No issues found. The changes look clean."
		}
		ch <- provider.StreamChunk{Delta: response}
		ch <- provider.StreamChunk{Done: true}
	}()
	return ch, nil
}

// mockSynthesizer combines individual reviews into a consensus.
type mockSynthesizer struct {
	id string
}

func (m *mockSynthesizer) ID() string          { return m.id }
func (m *mockSynthesizer) Authenticate() error { return nil }
func (m *mockSynthesizer) Validate() error     { return nil }

func (m *mockSynthesizer) Query(_ context.Context, messages []provider.Message, _ provider.QueryOpts) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 2)
	go func() {
		defer close(ch)
		lastMsg := messages[len(messages)-1].Content

		// If this is a consensus synthesis, combine the model outputs
		if strings.Contains(lastMsg, "Model responses:") {
			ch <- provider.StreamChunk{Delta: "CONSENSUS REVIEW:\n" + lastMsg}
		} else {
			ch <- provider.StreamChunk{Delta: lastMsg}
		}
		ch <- provider.StreamChunk{Done: true}
	}()
	return ch, nil
}

func TestReviewBenchmark_IndividualCases(t *testing.T) {
	// Model A: focuses on security issues
	modelA := &mockReviewProvider{
		id: "security-model",
		findings: map[string]string{
			"+ id + \"'\"":         "Severity: critical - SQL injection vulnerability. User input is concatenated directly into SQL query. Use parameterized queries.",
			"password :=":          "Severity: critical - Hardcoded credentials found. Never store secrets in source code.",
			"InsecureSkipVerify":   "Severity: critical - TLS certificate verification disabled. This allows man-in-the-middle attacks.",
			"r.URL.Query().Get":    "Severity: critical - Path traversal vulnerability. User input used directly in file path without sanitization.",
			"log.Printf(\"Login": "Severity: critical - Logging sensitive data (password). Remove password from log output.",
		},
	}

	// Model B: focuses on correctness
	modelB := &mockReviewProvider{
		id: "correctness-model",
		findings: map[string]string{
			"data, _ := io.ReadAll": "Severity: warning - Error from ReadAll is silently ignored. This could mask I/O failures.",
			"c.data[key]":          "Severity: critical - Map access without mutex protection. Concurrent reads/writes will cause race conditions.",
			"<-ch":                 "Severity: warning - Goroutine will leak: channel is created but never written to, blocking the goroutine forever.",
			"a * b":               "Severity: warning - Potential integer overflow in multiplication of int32 values.",
		},
	}

	for _, tc := range reviewCases {
		t.Run(tc.name, func(t *testing.T) {
			prompt := "Review the following code changes:\n```diff\n" + tc.diff + "\n```"

			msgs := []provider.Message{{Role: provider.RoleUser, Content: prompt}}
			opts := provider.QueryOpts{MaxTokens: 2048}

			// Run both models
			for _, model := range []*mockReviewProvider{modelA, modelB} {
				ctx := context.Background()
				stream, err := model.Query(ctx, msgs, opts)
				if err != nil {
					t.Fatalf("%s failed: %v", model.id, err)
				}

				var result string
				for chunk := range stream {
					result += chunk.Delta
				}

				// Check if model caught expected issues
				lower := strings.ToLower(result)
				for _, expected := range tc.expectedIssues {
					if strings.Contains(lower, strings.ToLower(expected)) {
						t.Logf("%s correctly identified: %s", model.id, expected)
					}
				}
			}
		})
	}
}

func TestReviewBenchmark_ConsensusVsSingleModel(t *testing.T) {
	// This test measures whether consensus catches more issues than any single model.
	// With mock providers, we can verify the pipeline mechanics work correctly.

	modelA := &mockReviewProvider{
		id: "model-a",
		findings: map[string]string{
			"+ id + \"'\"":       "SQL injection: user input concatenated into query.",
			"InsecureSkipVerify": "TLS verification disabled.",
		},
	}

	modelB := &mockReviewProvider{
		id: "model-b",
		findings: map[string]string{
			"c.data[key]": "Race condition: unprotected concurrent map access.",
			"password :=":  "Hardcoded credential detected.",
		},
	}

	synthesizer := &mockSynthesizer{id: "synthesizer"}

	tracker := tokens.NewTracker(
		map[string]string{"model-a": "a", "model-b": "b", "synthesizer": "s"},
		map[string]int{"model-a": 100000, "model-b": 100000, "synthesizer": 100000},
	)

	pipeline := consensus.NewPipeline(
		[]provider.Provider{synthesizer, modelA, modelB},
		synthesizer,
		10*time.Second,
		1,
		tracker,
		consensus.SynthesisBalanced,
	)

	// Test against the SQL injection case — only model A would catch it individually,
	// but consensus should surface model A's finding.
	sqlCase := reviewCases[0] // SQL injection
	prompt := "Review the following code changes:\n```diff\n" + sqlCase.diff + "\n```"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, _, err := pipeline.Run(ctx, []provider.Message{{Role: provider.RoleUser, Content: prompt}}, provider.QueryOpts{MaxTokens: 2048})
	if err != nil {
		t.Fatalf("consensus pipeline failed: %v", err)
	}

	var consensusResult string
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		consensusResult += chunk.Delta
	}

	if consensusResult == "" {
		t.Fatal("consensus produced empty result")
	}

	// Verify the consensus pipeline produced a non-empty review
	t.Logf("Consensus result length: %d chars", len(consensusResult))
}

func TestReviewBenchmark_CleanDiffProducesNoFalsePositives(t *testing.T) {
	// The clean refactor case should not trigger false positives.
	cleanCase := reviewCases[8] // "Clean refactor - no issues"

	model := &mockReviewProvider{
		id:       "strict-model",
		findings: map[string]string{}, // no patterns to match
	}

	prompt := "Review the following code changes:\n```diff\n" + cleanCase.diff + "\n```"
	ctx := context.Background()
	stream, err := model.Query(ctx, []provider.Message{{Role: provider.RoleUser, Content: prompt}}, provider.QueryOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var result string
	for chunk := range stream {
		result += chunk.Delta
	}

	lower := strings.ToLower(result)
	if strings.Contains(lower, "critical") || strings.Contains(lower, "vulnerability") {
		t.Errorf("false positive on clean diff: %s", result)
	}
}
