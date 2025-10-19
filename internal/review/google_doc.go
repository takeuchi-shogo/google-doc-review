package review

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/option"
)

// GoogleDocFetcher handles fetching content from Google Docs
type GoogleDocFetcher struct {
	client *http.Client
}

// NewGoogleDocFetcher creates a new GoogleDocFetcher
func NewGoogleDocFetcher(client *http.Client) *GoogleDocFetcher {
	return &GoogleDocFetcher{
		client: client,
	}
}

// Document represents a Google Doc with its content
type Document struct {
	ID      string
	Title   string
	Content string
}

// ExtractDocumentID extracts the document ID from a Google Docs URL
// Supports URLs like:
// - https://docs.google.com/document/d/{documentId}/edit
// - https://docs.google.com/document/d/{documentId}
func ExtractDocumentID(url string) (string, error) {
	// Regular expression to match Google Docs URL pattern
	pattern := `docs\.google\.com/document/d/([a-zA-Z0-9-_]+)`
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid Google Docs URL: %s", url)
	}

	return matches[1], nil
}

// FetchDocument fetches a Google Doc by URL and returns its content
func (f *GoogleDocFetcher) FetchDocument(ctx context.Context, url string) (*Document, error) {
	// Extract document ID from URL
	docID, err := ExtractDocumentID(url)
	if err != nil {
		return nil, err
	}

	return f.FetchDocumentByID(ctx, docID)
}

// FetchDocumentByID fetches a Google Doc by its ID and returns its content
func (f *GoogleDocFetcher) FetchDocumentByID(ctx context.Context, documentID string) (*Document, error) {
	// Create Docs service
	docsService, err := docs.NewService(ctx, option.WithHTTPClient(f.client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Docs service: %w", err)
	}

	// Fetch the document
	doc, err := docsService.Documents.Get(documentID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document: %w", err)
	}

	// Extract text content from the document
	content := extractTextFromDocument(doc)

	return &Document{
		ID:      documentID,
		Title:   doc.Title,
		Content: content,
	}, nil
}

// extractTextFromDocument extracts plain text content from a Google Doc
func extractTextFromDocument(doc *docs.Document) string {
	var builder strings.Builder

	if doc.Body == nil || doc.Body.Content == nil {
		return ""
	}

	for _, element := range doc.Body.Content {
		extractTextFromStructuralElement(element, &builder)
	}

	return builder.String()
}

// extractTextFromStructuralElement recursively extracts text from structural elements
func extractTextFromStructuralElement(element *docs.StructuralElement, builder *strings.Builder) {
	if element.Paragraph != nil {
		extractTextFromParagraph(element.Paragraph, builder)
	}

	if element.Table != nil {
		extractTextFromTable(element.Table, builder)
	}

	if element.SectionBreak != nil {
		builder.WriteString("\n---\n")
	}
}

// extractTextFromParagraph extracts text from a paragraph
func extractTextFromParagraph(paragraph *docs.Paragraph, builder *strings.Builder) {
	if paragraph.Elements == nil {
		return
	}

	for _, elem := range paragraph.Elements {
		if elem.TextRun != nil && elem.TextRun.Content != "" {
			builder.WriteString(elem.TextRun.Content)
		}
	}
}

// extractTextFromTable extracts text from a table
func extractTextFromTable(table *docs.Table, builder *strings.Builder) {
	if table.TableRows == nil {
		return
	}

	for _, row := range table.TableRows {
		if row.TableCells == nil {
			continue
		}

		for i, cell := range row.TableCells {
			if cell.Content != nil {
				for _, element := range cell.Content {
					extractTextFromStructuralElement(element, builder)
				}
			}

			// Add separator between cells (except last cell in row)
			if i < len(row.TableCells)-1 {
				builder.WriteString(" | ")
			}
		}
		builder.WriteString("\n")
	}
}
