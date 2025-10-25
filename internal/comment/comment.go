package comment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// CommentManager handles creating and managing comments on Google Docs
type CommentManager struct {
	client       *http.Client
	driveService *drive.Service
	docsService  *docs.Service
}

// NewCommentManager creates a new CommentManager
func NewCommentManager(client *http.Client) (*CommentManager, error) {
	ctx := context.Background()
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive service: %w", err)
	}

	docsService, err := docs.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Docs service: %w", err)
	}

	return &CommentManager{
		client:       client,
		driveService: driveService,
		docsService:  docsService,
	}, nil
}

// CommentRequest represents a request to create a comment
type CommentRequest struct {
	FileID          string // Google Doc file ID
	Content         string // Comment content
	QuotedText      string // Text to quote (optional)
	LineNumber      int    // Line number for anchored comment (optional, 0 means no anchor)
	LineLength      int    // Length of the line selection (optional)
}

// CommentResponse represents the result of creating a comment
type CommentResponse struct {
	CommentID string
	Content   string
	Anchor    string
	CreatedAt string
}

// CreateComment creates a comment on a Google Doc with automatic anchor if quoted text is provided
func (cm *CommentManager) CreateComment(ctx context.Context, req *CommentRequest) (*CommentResponse, error) {
	comment := &drive.Comment{
		Content: req.Content,
	}

	// If quoted text is provided, try to find its position and create an anchor
	if req.QuotedText != "" {
		pos, err := cm.FindTextPosition(ctx, req.FileID, req.QuotedText)
		if err == nil {
			// Try creating anchor with position
			anchor, err := createAnchorJSONWithPosition(pos)
			if err == nil {
				comment.Anchor = anchor
			}
		}

		// Also add quoted text for display
		comment.QuotedFileContent = &drive.CommentQuotedFileContent{
			MimeType: "text/plain",
			Value:    req.QuotedText,
		}
	}

	// Create the comment
	createdComment, err := cm.driveService.Comments.
		Create(req.FileID, comment).
		Context(ctx).
		Fields("id,content,createdTime,anchor,quotedFileContent").
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return &CommentResponse{
		CommentID: createdComment.Id,
		Content:   createdComment.Content,
		Anchor:    createdComment.Anchor,
		CreatedAt: createdComment.CreatedTime,
	}, nil
}

// CreateAnchoredComment creates an anchored comment on a specific line in a Google Doc
func (cm *CommentManager) CreateAnchoredComment(ctx context.Context, req *CommentRequest) (*CommentResponse, error) {
	if req.LineNumber <= 0 {
		return nil, fmt.Errorf("line number must be greater than 0 for anchored comments")
	}

	// Create anchor JSON structure using line number
	anchor, err := createAnchorJSON(req.LineNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to create anchor: %w", err)
	}

	comment := &drive.Comment{
		Content: req.Content,
		Anchor:  anchor,
	}

	// Add quoted text for display if provided
	if req.QuotedText != "" {
		comment.QuotedFileContent = &drive.CommentQuotedFileContent{
			MimeType: "text/plain",
			Value:    req.QuotedText,
		}
	}

	// Create the comment
	createdComment, err := cm.driveService.Comments.
		Create(req.FileID, comment).
		Context(ctx).
		Fields("id,content,createdTime,anchor,quotedFileContent").
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to create anchored comment: %w", err)
	}

	return &CommentResponse{
		CommentID: createdComment.Id,
		Content:   createdComment.Content,
		Anchor:    createdComment.Anchor,
		CreatedAt: createdComment.CreatedTime,
	}, nil
}

// CreateMultipleComments creates multiple comments in batch
func (cm *CommentManager) CreateMultipleComments(ctx context.Context, requests []*CommentRequest) ([]*CommentResponse, error) {
	responses := make([]*CommentResponse, 0, len(requests))
	errors := make([]error, 0)

	for i, req := range requests {
		var resp *CommentResponse
		var err error

		// Use anchored comment if line number is specified
		if req.LineNumber > 0 {
			resp, err = cm.CreateAnchoredComment(ctx, req)
		} else {
			resp, err = cm.CreateComment(ctx, req)
		}

		if err != nil {
			errors = append(errors, fmt.Errorf("comment %d: %w", i, err))
			continue
		}

		responses = append(responses, resp)
	}

	if len(errors) > 0 {
		return responses, fmt.Errorf("failed to create %d comments: %v", len(errors), errors)
	}

	return responses, nil
}

// ListComments lists all comments on a Google Doc
func (cm *CommentManager) ListComments(ctx context.Context, fileID string) ([]*drive.Comment, error) {
	commentList, err := cm.driveService.Comments.
		List(fileID).
		Context(ctx).
		Fields("comments(id,content,createdTime,anchor,quotedFileContent,author)").
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	return commentList.Comments, nil
}

// DeleteComment deletes a comment from a Google Doc
func (cm *CommentManager) DeleteComment(ctx context.Context, fileID, commentID string) error {
	err := cm.driveService.Comments.
		Delete(fileID, commentID).
		Context(ctx).
		Do()

	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

// createAnchorJSON creates the anchor JSON string for Drive API
// Deprecated: Use createAnchorJSONWithPosition instead
func createAnchorJSON(lineNumber int) (string, error) {
	anchor := map[string]any{
		"region": map[string]any{
			"kind": "drive#commentRegion",
			"line": lineNumber,
			"rev":  "head", // Always use "head" for the latest revision
		},
	}

	anchorJSON, err := json.Marshal(anchor)
	if err != nil {
		return "", fmt.Errorf("failed to marshal anchor: %w", err)
	}

	return string(anchorJSON), nil
}

// createAnchorJSONWithPosition creates anchor JSON using text position
func createAnchorJSONWithPosition(pos *TextPosition) (string, error) {
	anchor := map[string]any{
		"region": map[string]any{
			"startIndex": pos.StartIndex,
			"endIndex":   pos.EndIndex,
		},
	}

	anchorJSON, err := json.Marshal(anchor)
	if err != nil {
		return "", fmt.Errorf("failed to marshal anchor: %w", err)
	}

	return string(anchorJSON), nil
}

// IssueType represents the type of issue found in a document
type IssueType string

const (
	IssueTypeGrammar    IssueType = "grammar"
	IssueTypeClarity    IssueType = "clarity"
	IssueTypeStructure  IssueType = "structure"
	IssueTypeMissing    IssueType = "missing"
	IssueTypeInconsistent IssueType = "inconsistent"
)

// IssueSeverity represents how critical an issue is
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "critical"
	SeverityWarning  IssueSeverity = "warning"
	SeverityInfo     IssueSeverity = "info"
)

// Issue represents a problem found in a document
type Issue struct {
	Type        IssueType
	Severity    IssueSeverity
	LineNumber  int
	TextContent string
	Suggestion  string
	Description string
}

// CreateCommentsFromIssues converts a list of issues into comments
func (cm *CommentManager) CreateCommentsFromIssues(ctx context.Context, fileID string, issues []Issue) ([]*CommentResponse, error) {
	requests := make([]*CommentRequest, 0, len(issues))

	for _, issue := range issues {
		// Format comment content
		content := formatIssueComment(issue)

		req := &CommentRequest{
			FileID:     fileID,
			Content:    content,
			QuotedText: issue.TextContent,
			LineNumber: issue.LineNumber,
			LineLength: 1, // Default length
		}

		requests = append(requests, req)
	}

	return cm.CreateMultipleComments(ctx, requests)
}

// formatIssueComment formats an issue into a readable comment
func formatIssueComment(issue Issue) string {
	emoji := map[IssueSeverity]string{
		SeverityCritical: "🔴",
		SeverityWarning:  "⚠️",
		SeverityInfo:     "ℹ️",
	}

	return fmt.Sprintf("%s %s: %s\n\n%s",
		emoji[issue.Severity],
		issue.Type,
		issue.Description,
		issue.Suggestion,
	)
}

// TextPosition represents a text location in a document
type TextPosition struct {
	StartIndex int64
	EndIndex   int64
	SegmentID  string
}

// FindTextPosition searches for text in a document and returns its position
func (cm *CommentManager) FindTextPosition(ctx context.Context, fileID, searchText string) (*TextPosition, error) {
	// Get the document content
	doc, err := cm.docsService.Documents.Get(fileID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Search through document content
	for _, element := range doc.Body.Content {
		if paragraph := element.Paragraph; paragraph != nil {
			for _, paragraphElement := range paragraph.Elements {
				if textRun := paragraphElement.TextRun; textRun != nil {
					// Check if this text contains our search text
					if strings.Contains(textRun.Content, searchText) {
						// Calculate the exact position of the search text
						offset := int64(strings.Index(textRun.Content, searchText))
						return &TextPosition{
							StartIndex: paragraphElement.StartIndex + offset,
							EndIndex:   paragraphElement.StartIndex + offset + int64(len(searchText)),
							SegmentID:  "",
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("text not found: %s", searchText)
}
