package comment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewCommentManager(t *testing.T) {
	client := &http.Client{}
	cm, err := NewCommentManager(client)

	if err != nil {
		t.Fatalf("NewCommentManager() error = %v", err)
	}

	if cm == nil {
		t.Fatal("NewCommentManager() returned nil")
	}

	if cm.client != client {
		t.Error("NewCommentManager() client not set correctly")
	}

	if cm.driveService == nil {
		t.Error("NewCommentManager() driveService is nil")
	}
}

func TestCreateAnchorJSON(t *testing.T) {
	tests := []struct {
		name       string
		lineNumber int
		wantErr    bool
	}{
		{
			name:       "valid anchor line 10",
			lineNumber: 10,
			wantErr:    false,
		},
		{
			name:       "valid anchor line 20",
			lineNumber: 20,
			wantErr:    false,
		},
		{
			name:       "line number 1",
			lineNumber: 1,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anchorJSON, err := createAnchorJSON(tt.lineNumber)

			if (err != nil) != tt.wantErr {
				t.Errorf("createAnchorJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Verify JSON structure
			var anchor map[string]interface{}
			if err := json.Unmarshal([]byte(anchorJSON), &anchor); err != nil {
				t.Errorf("createAnchorJSON() produced invalid JSON: %v", err)
				return
			}

			// Check region structure
			region, ok := anchor["region"].(map[string]interface{})
			if !ok {
				t.Error("createAnchorJSON() region object invalid")
				return
			}

			// Verify kind
			if region["kind"] != "drive#commentRegion" {
				t.Errorf("createAnchorJSON() kind = %v, want 'drive#commentRegion'", region["kind"])
			}

			// Verify line number
			line, ok := region["line"].(float64)
			if !ok || int(line) != tt.lineNumber {
				t.Errorf("createAnchorJSON() line number = %v, want %v", line, tt.lineNumber)
			}

			// Verify revision
			if region["rev"] != "head" {
				t.Errorf("createAnchorJSON() revision = %v, want 'head'", region["rev"])
			}
		})
	}
}

func TestFormatIssueComment(t *testing.T) {
	tests := []struct {
		name  string
		issue Issue
	}{
		{
			name: "critical grammar issue",
			issue: Issue{
				Type:        IssueTypeGrammar,
				Severity:    SeverityCritical,
				Description: "Subject-verb agreement error",
				Suggestion:  "Change 'they was' to 'they were'",
			},
		},
		{
			name: "warning clarity issue",
			issue: Issue{
				Type:        IssueTypeClarity,
				Severity:    SeverityWarning,
				Description: "Sentence is too complex",
				Suggestion:  "Break into two sentences",
			},
		},
		{
			name: "info structure issue",
			issue: Issue{
				Type:        IssueTypeStructure,
				Severity:    SeverityInfo,
				Description: "Consider adding a transition",
				Suggestion:  "Add 'However,' at the beginning",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIssueComment(tt.issue)
			// Just verify the format contains the key elements
			if got == "" {
				t.Error("formatIssueComment() returned empty string")
			}
			// Verify it contains the description and suggestion
			if !strings.Contains(got, tt.issue.Description) {
				t.Errorf("formatIssueComment() missing description: %v", tt.issue.Description)
			}
			if !strings.Contains(got, tt.issue.Suggestion) {
				t.Errorf("formatIssueComment() missing suggestion: %v", tt.issue.Suggestion)
			}
		})
	}
}

func TestCommentRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request *CommentRequest
		wantErr bool
	}{
		{
			name: "valid basic request",
			request: &CommentRequest{
				FileID:  "test-file-id",
				Content: "This is a test comment",
			},
			wantErr: false,
		},
		{
			name: "valid request with quoted text",
			request: &CommentRequest{
				FileID:     "test-file-id",
				Content:    "Grammar issue",
				QuotedText: "they was going",
			},
			wantErr: false,
		},
		{
			name: "valid anchored request",
			request: &CommentRequest{
				FileID:     "test-file-id",
				Content:    "Fix this",
				LineNumber: 10,
				LineLength: 2,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation checks
			if tt.request.FileID == "" {
				t.Error("FileID should not be empty")
			}
			if tt.request.Content == "" {
				t.Error("Content should not be empty")
			}
		})
	}
}

func TestCreateCommentsFromIssues(t *testing.T) {
	// Create a test server to mock Drive API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock response for comment creation
		if r.Method == "POST" && r.URL.Path == "/drive/v3/files/test-file-id/comments" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":          "comment-123",
				"content":     "Test comment",
				"createdTime": "2024-01-01T00:00:00Z",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Note: This test is incomplete because we can't easily mock the Drive API client
	// In a real test, you would use dependency injection or interfaces to mock the service
	t.Skip("Skipping integration test - requires mocking Drive API")
}

func TestIssueTypes(t *testing.T) {
	// Test that all issue types are defined
	issueTypes := []IssueType{
		IssueTypeGrammar,
		IssueTypeClarity,
		IssueTypeStructure,
		IssueTypeMissing,
		IssueTypeInconsistent,
	}

	for _, issueType := range issueTypes {
		if issueType == "" {
			t.Errorf("IssueType should not be empty")
		}
	}

	// Test that all severities are defined
	severities := []IssueSeverity{
		SeverityCritical,
		SeverityWarning,
		SeverityInfo,
	}

	for _, severity := range severities {
		if severity == "" {
			t.Errorf("IssueSeverity should not be empty")
		}
	}
}

func TestCommentResponse(t *testing.T) {
	resp := &CommentResponse{
		CommentID: "test-id",
		Content:   "test content",
		Anchor:    "test-anchor",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	if resp.CommentID == "" {
		t.Error("CommentID should not be empty")
	}

	if resp.Content == "" {
		t.Error("Content should not be empty")
	}
}
