package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/jomei/notionapi"
)

type Field struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    string      `json:"required"`
	Format      string      `json:"format"`
	Description string      `json:"description"`
	Children    []Field     `json:"children,omitempty"`
	Value       interface{} `json:"value,omitempty"`
}

func formatRequiredField(required string) string {
	if required == "Required" {
		return "✅"
	}
	return "❌"
}

func formatType(fieldType string) string {
	switch fieldType {
	case "object":
		return "{}obj"
	case "array_object":
		return "[]obj"
	case "int", "int64", "int32", "uint64", "uint32", "integer":
		return "int"
	case "time.Time":
		return "datetime"
	case "bool":
		return "boolean"
	default:
		// Handle array types
		if strings.HasPrefix(fieldType, "[]") {
			baseType := strings.TrimPrefix(fieldType, "[]")
			// Format the base type and keep the array notation
			switch baseType {
			case "int", "int64", "int32", "uint64", "uint32", "integer":
				return "[]int"
			case "string":
				return "[]string"
			case "bool", "boolean":
				return "[]boolean"
			default:
				return fieldType // Return as is for other array types
			}
		}
		return fieldType
	}
}

func formatArrayType(fieldType string, isArray bool) string {
	formattedType := formatType(fieldType)
	if isArray {
		return "[]" + formattedType
	}
	return formattedType
}

func generateExampleValue(fieldType string, format string, children []Field) interface{} {
	// Handle array types first
	if strings.HasPrefix(fieldType, "[]") {
		baseType := strings.TrimPrefix(fieldType, "[]")
		// Generate an example array with one item of the base type
		switch baseType {
		case "int", "int64", "int32", "uint64", "uint32", "integer":
			return []interface{}{0}
		case "string":
			if format != "" {
				return []interface{}{generateFormattedStringExample(format)}
			}
			return []interface{}{"example_string"}
		case "bool", "boolean":
			return []interface{}{false}
		default:
			return []interface{}{nil}
		}
	}

	// Handle non-array types
	switch fieldType {
	case "string":
		if format != "" {
			return generateFormattedStringExample(format)
		}
		return "example_string"
	case "int", "int64", "int32", "uint64", "uint32", "integer":
		return 0
	case "bool", "boolean":
		return false
	case "time.Time":
		return "2024-09-01T14:26:00+09:00"
	case "object":
		if len(children) > 0 {
			childExample := make(map[string]interface{})
			for _, child := range children {
				childExample[child.Name] = generateExampleValue(child.Type, child.Format, child.Children)
			}
			return childExample
		}
		return make(map[string]interface{})
	case "array_object":
		if len(children) > 0 {
			childExample := make(map[string]interface{})
			for _, child := range children {
				childExample[child.Name] = generateExampleValue(child.Type, child.Format, child.Children)
			}
			return []interface{}{childExample}
		}
		return []interface{}{make(map[string]interface{})}
	default:
		return nil
	}
}

// Helper function to generate formatted string examples
func generateFormattedStringExample(format string) string {
	switch format {
	case "YYYYMMDD":
		return "20240901"
	case "HHMM":
		return "14:26:00"
	case "ISO8601":
		return "2024-09-01T14:26:00+09:00"
	default:
		return "example_string_with_format_" + format
	}
}

// New function to generate example JSON based on field definitions
func generateExampleJSON(fields []Field) ([]byte, error) {
	exampleMap := make(map[string]interface{})
	for _, field := range fields {
		exampleMap[field.Name] = generateExampleValue(field.Type, field.Format, field.Children)
	}
	return json.MarshalIndent(exampleMap, "", "    ") // Use 4 spaces
}

// Updated helper function to parse field data recursively and handle array types correctly
func parseFieldData(name string, fieldData map[string]interface{}) Field {
	log.Printf("Parsing field: %s, type: %v", name, fieldData["type"])

	originalFieldType := ""
	if ft, ok := fieldData["type"].(string); ok {
		originalFieldType = ft
	}

	finalFieldType := originalFieldType
	children := []Field{}
	format := ""
	description := ""
	required := fmt.Sprintf("%v", fieldData["required"])

	if f, ok := fieldData["format"].(string); ok {
		format = f
	}
	if d, ok := fieldData["description"].(string); ok {
		description = d
	}

	// Handle complex types (array, object)
	if originalFieldType == "array" {
		log.Printf("Processing array field: %s", name)
		if itemsData, ok := fieldData["items"].(map[string]interface{}); ok {
			itemType := ""
			if it, ok := itemsData["type"].(string); ok {
				itemType = it
			}
			log.Printf("Array item type for %s: %s", name, itemType)

			// Check if items define an object (explicitly or via properties)
			if itemType == "object" || itemsData["properties"] != nil {
				finalFieldType = "array_object"
				if properties, ok := itemsData["properties"].(map[string]interface{}); ok {
					for propName, propData := range properties {
						if propMap, ok := propData.(map[string]interface{}); ok {
							log.Printf("Processing array object property: %s", propName)
							children = append(children, parseFieldData(propName, propMap))
						}
					}
				}
			} else if itemType != "" {
				// Handle integer types specifically
				if itemType == "integer" || strings.HasSuffix(itemType, "int") || strings.HasSuffix(itemType, "int32") || strings.HasSuffix(itemType, "int64") {
					finalFieldType = "[]integer"
					log.Printf("Set array integer type for %s", name)
				} else {
					// Array of other simple types
					finalFieldType = "[]" + itemType
					log.Printf("Set array type for %s: %s", name, finalFieldType)
				}
			} else {
				finalFieldType = "[]unknown"
			}
		}
	} else if originalFieldType == "object" {
		log.Printf("Processing object field: %s", name)
		// Handle nested object properties
		if properties, ok := fieldData["properties"].(map[string]interface{}); ok {
			log.Printf("Found properties in object %s", name)
			for propName, propData := range properties {
				if propMap, ok := propData.(map[string]interface{}); ok {
					log.Printf("Processing object property: %s in %s", propName, name)
					children = append(children, parseFieldData(propName, propMap))
				}
			}
		}
	}

	log.Printf("Completed parsing field %s with type %s", name, finalFieldType)
	return Field{
		Name:        name,
		Type:        finalFieldType,
		Required:    required,
		Format:      format,
		Description: description,
		Children:    children,
	}
}

func main() {
	// Define and parse command-line flags
	pageTitle := flag.String("title", "JSON Schema Documentation", "The title for the Notion page")
	flag.Parse()

	// Read input.json
	jsonData, err := ioutil.ReadFile("input.json")
	if err != nil {
		log.Fatalf("Error reading input.json: %v", err)
	}

	// Parse JSON into sections
	var data map[string]map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	// Initialize Notion client
	client := notionapi.NewClient(notionapi.Token("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"))

	// Create blocks for each section
	var blocks []notionapi.Block

	// Add special characters text block
	blocks = append(blocks, &notionapi.ParagraphBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: "block",
			Type:   "paragraph",
		},
		Paragraph: notionapi.Paragraph{
			RichText: []notionapi.RichText{
				{
					Text: &notionapi.Text{
						Content: "Special characters: —— ❌ —— ✅︎ ——",
					},
				},
			},
		},
	})

	// Add Change Histories toggle
	blocks = append(blocks, &notionapi.ToggleBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: "block",
			Type:   "toggle",
		},
		Toggle: notionapi.Toggle{
			RichText: []notionapi.RichText{
				{
					Text: &notionapi.Text{
						Content: "Change Histories",
					},
					Annotations: &notionapi.Annotations{
						Code:  true,
						Bold:  true,
						Color: "red",
					},
				},
			},
			Children: []notionapi.Block{
				&notionapi.TableBlock{
					BasicBlock: notionapi.BasicBlock{
						Object: "block",
						Type:   "table",
					},
					Table: notionapi.Table{
						TableWidth:      4,
						HasColumnHeader: true,
						HasRowHeader:    false,
						Children: []notionapi.Block{
							&notionapi.TableRowBlock{
								BasicBlock: notionapi.BasicBlock{
									Object: "block",
									Type:   "table_row",
								},
								TableRow: notionapi.TableRow{
									Cells: [][]notionapi.RichText{
										{{Text: &notionapi.Text{Content: "Task"}}},
										{{Text: &notionapi.Text{Content: "Date"}}},
										{{Text: &notionapi.Text{Content: "Description"}}},
										{{Text: &notionapi.Text{Content: "Need FE fix?"}}},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	// Add spacing
	blocks = append(blocks, &notionapi.ParagraphBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: "block",
			Type:   "paragraph",
		},
		Paragraph: notionapi.Paragraph{
			RichText: []notionapi.RichText{},
		},
	})

	sections := []string{"param", "query", "request_body", "response"}
	sectionTitles := map[string]string{
		"param":        "1. Path Parameters",
		"query":        "2. Query Parameters",
		"request_body": "3. Request Body",
		"response":     "4. Response",
	}

	for _, section := range sections {
		// Add heading for the section
		blocks = append(blocks, &notionapi.Heading1Block{
			BasicBlock: notionapi.BasicBlock{
				Object: "block",
				Type:   "heading_1",
			},
			Heading1: notionapi.Heading{
				RichText: []notionapi.RichText{
					{
						Text: &notionapi.Text{
							Content: sectionTitles[section],
						},
					},
				},
			},
		})

		// Parse fields for this section
		var sectionFields []Field
		if sectionData, ok := data[section]; ok {
			// First, check for fields inside the "fields" object
			if fields, ok := sectionData["fields"].(map[string]interface{}); ok {
				for name, field := range fields {
					if fieldData, ok := field.(map[string]interface{}); ok {
						sectionFields = append(sectionFields, parseFieldData(name, fieldData))
					}
				}
			}

			// Then, check for fields directly in the section root
			// Skip known metadata keys that aren't actual fields
			skipKeys := map[string]bool{
				"fields": true,
				"type":   true, // Skip if the section itself has a type
			}

			for name, field := range sectionData {
				if !skipKeys[name] {
					if fieldData, ok := field.(map[string]interface{}); ok {
						// Check if this is actually a field definition (should have a "type" key)
						if _, hasType := fieldData["type"]; hasType {
							sectionFields = append(sectionFields, parseFieldData(name, fieldData))
						}
					}
				}
			}
		}

		// Create table for this section
		if len(sectionFields) > 0 {
			blocks = append(blocks, &notionapi.TableBlock{
				BasicBlock: notionapi.BasicBlock{
					Object: "block",
					Type:   "table",
				},
				Table: notionapi.Table{
					TableWidth:      6,
					HasColumnHeader: true,
					HasRowHeader:    false,
					Children:        createTableRows(sectionFields),
				},
			})
		} else {
			// Create an empty table with headers and an empty message row
			blocks = append(blocks, &notionapi.TableBlock{
				BasicBlock: notionapi.BasicBlock{
					Object: "block",
					Type:   "table",
				},
				Table: notionapi.Table{
					TableWidth:      6,
					HasColumnHeader: true,
					HasRowHeader:    false,
					Children: []notionapi.Block{
						// Header Row
						&notionapi.TableRowBlock{
							BasicBlock: notionapi.BasicBlock{
								Object: "block",
								Type:   "table_row",
							},
							TableRow: notionapi.TableRow{
								Cells: [][]notionapi.RichText{
									{{Text: &notionapi.Text{Content: "Name"}}},
									{{Text: &notionapi.Text{Content: ""}}},
									{{Text: &notionapi.Text{Content: "Type"}}},
									{{Text: &notionapi.Text{Content: "Required"}}},
									{{Text: &notionapi.Text{Content: "Format"}}},
									{{Text: &notionapi.Text{Content: "Description"}}},
								},
							},
						},
						// Empty Message Row
						&notionapi.TableRowBlock{
							BasicBlock: notionapi.BasicBlock{
								Object: "block",
								Type:   "table_row",
							},
							TableRow: notionapi.TableRow{
								Cells: [][]notionapi.RichText{
									{{
										Text: &notionapi.Text{Content: "No fields defined for this section"},
										Annotations: &notionapi.Annotations{
											Italic: true,
											Color:  "gray",
										},
									}},
									{{Text: &notionapi.Text{Content: ""}}},
									{{Text: &notionapi.Text{Content: ""}}},
									{{Text: &notionapi.Text{Content: ""}}},
									{{Text: &notionapi.Text{Content: ""}}},
									{{Text: &notionapi.Text{Content: ""}}},
								},
							},
						},
					},
				},
			})
		}

		// Add Example block using generated JSON based on field definitions
		if (section == "request_body" || section == "response") && len(sectionFields) > 0 { // Check if fields were parsed
			// Add "Example:" text block
			blocks = append(blocks, &notionapi.ParagraphBlock{
				BasicBlock: notionapi.BasicBlock{
					Object: "block",
					Type:   "paragraph",
				},
				Paragraph: notionapi.Paragraph{
					RichText: []notionapi.RichText{
						{
							Text: &notionapi.Text{
								Content: "Example:",
							},
							Annotations: &notionapi.Annotations{
								Bold: true,
							},
						},
					},
				},
			})

			// Generate the example JSON from the field definitions
			exampleJSON, err := generateExampleJSON(sectionFields)
			if err != nil {
				log.Printf("Warning: could not generate example JSON for section %s: %v", section, err)
			} else {
				// Split content if it exceeds the 2000 character limit
				content := string(exampleJSON)
				const limit = 2000
				richTextElements := []notionapi.RichText{}

				if len(content) > limit {
					for i := 0; i < len(content); i += limit {
						end := i + limit
						if end > len(content) {
							end = len(content)
						}
						richTextElements = append(richTextElements, notionapi.RichText{
							Text: &notionapi.Text{
								Content: content[i:end],
							},
						})
					}
				} else {
					richTextElements = append(richTextElements, notionapi.RichText{
						Text: &notionapi.Text{
							Content: content,
						},
					})
				}

				blocks = append(blocks, &notionapi.CodeBlock{
					BasicBlock: notionapi.BasicBlock{
						Object: "block",
						Type:   "code",
					},
					Code: notionapi.Code{
						Language: "json",
						RichText: richTextElements,
					},
				})
			}
		}

		// Add spacing
		blocks = append(blocks, &notionapi.ParagraphBlock{
			BasicBlock: notionapi.BasicBlock{
				Object: "block",
				Type:   "paragraph",
			},
			Paragraph: notionapi.Paragraph{
				RichText: []notionapi.RichText{},
			},
		})
	}

	// Create page request
	pageReq := &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			DatabaseID: notionapi.DatabaseID("1d863735-e3ae-802e-afd5-d4ed68b646cf"),
		},
		Properties: notionapi.Properties{
			"Name": notionapi.TitleProperty{
				Title: []notionapi.RichText{
					{
						Text: &notionapi.Text{
							Content: *pageTitle,
						},
					},
				},
			},
		},
		Children: blocks,
	}

	// Create the page
	page, err := client.Page.Create(context.Background(), pageReq)
	if err != nil {
		log.Fatalf("Error creating page: %v", err)
	}

	fmt.Printf("Page created successfully with ID: %s\n", page.ID)
}

func parseJSONWithComments(jsonStr string) []Field {
	// Remove comments from JSON
	lines := strings.Split(jsonStr, "\n")
	var cleanLines []string
	for _, line := range lines {
		if idx := strings.Index(line, "//"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	cleanJSON := strings.Join(cleanLines, "\n")

	// Parse the cleaned JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(cleanJSON), &data); err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	// Extract field information from comments
	fields := make([]Field, 0)
	for key, value := range data {
		field := Field{
			Name: key,
		}

		// Find the original line with comments
		for _, line := range lines {
			if strings.Contains(line, key) {
				// Extract type and required information from comment
				if strings.Contains(line, "Required") {
					field.Required = "Required"
				} else if strings.Contains(line, "Optional") {
					field.Required = "Optional"
				}

				// Extract type information
				if strings.Contains(line, "string") {
					field.Type = "string"
				} else if strings.Contains(line, "uint64") {
					field.Type = "uint64"
				} else if strings.Contains(line, "int64") {
					field.Type = "int64"
				} else if strings.Contains(line, "bool") {
					field.Type = "bool"
				} else if strings.Contains(line, "time.Time") {
					field.Type = "time.Time"
				} else if strings.Contains(line, "object") {
					field.Type = "object"
				}

				// Extract format information
				if strings.Contains(line, "YYYYMMDD") {
					field.Format = "YYYYMMDD"
				} else if strings.Contains(line, "HHMM") {
					field.Format = "HHMM"
				} else if strings.Contains(line, "ISO8601") {
					field.Format = "ISO8601"
				}

				// Extract description
				if idx := strings.Index(line, "-"); idx != -1 {
					field.Description = strings.TrimSpace(line[idx+1:])
				}

				break
			}
		}

		// Handle nested objects
		if nestedMap, ok := value.(map[string]interface{}); ok {
			for nestedKey := range nestedMap {
				field.Children = append(field.Children, Field{
					Name: nestedKey,
					Type: "string",
				})
			}
		}

		fields = append(fields, field)
	}

	return fields
}

func createTableRows(fields []Field) []notionapi.Block {
	// Create header row
	rows := []notionapi.Block{
		&notionapi.TableRowBlock{
			BasicBlock: notionapi.BasicBlock{
				Object: "block",
				Type:   "table_row",
			},
			TableRow: notionapi.TableRow{
				Cells: [][]notionapi.RichText{
					{{Text: &notionapi.Text{Content: "Name"}}},
					{{Text: &notionapi.Text{Content: ""}}},
					{{Text: &notionapi.Text{Content: "Type"}}},
					{{Text: &notionapi.Text{Content: "Required"}}},
					{{Text: &notionapi.Text{Content: "Format"}}},
					{{Text: &notionapi.Text{Content: "Description"}}},
				},
			},
		},
	}

	// Create data rows
	for _, field := range fields {
		// Add parent field row
		rows = append(rows, &notionapi.TableRowBlock{
			BasicBlock: notionapi.BasicBlock{
				Object: "block",
				Type:   "table_row",
			},
			TableRow: notionapi.TableRow{
				Cells: [][]notionapi.RichText{
					{{
						Text: &notionapi.Text{
							Content: field.Name,
						},
						Annotations: &notionapi.Annotations{
							Code:  true,
							Color: "red",
						},
					}},
					{{Text: &notionapi.Text{Content: ""}}},
					{{Text: &notionapi.Text{Content: formatType(field.Type)}}},
					{{Text: &notionapi.Text{Content: formatRequiredField(field.Required)}}},
					{{Text: &notionapi.Text{Content: field.Format}}},
					{{Text: &notionapi.Text{Content: field.Description}}},
				},
			},
		})

		// For fields with children, create separate rows for each child
		if len(field.Children) > 0 {
			for _, child := range field.Children {
				// Determine how to format the child's type based on the parent's type
				var childTypeFormatted string
				if field.Type == "array_object" {
					// Parent is an array of objects, child is an item's field
					// We should still format the child's type directly based on its own type definition
					// E.g., if child.Type is "string", display "string", not "[]string"
					// If child.Type itself is an array (e.g., "[]string"), formatType should handle it
					childTypeFormatted = formatType(child.Type)
				} else {
					// Parent is likely a regular object, child is a direct field
					childTypeFormatted = formatType(child.Type)
				}

				rows = append(rows, &notionapi.TableRowBlock{
					BasicBlock: notionapi.BasicBlock{
						Object: "block",
						Type:   "table_row",
					},
					TableRow: notionapi.TableRow{
						Cells: [][]notionapi.RichText{
							{{Text: &notionapi.Text{Content: ""}}}, // Indent child name
							{{
								Text: &notionapi.Text{
									Content: child.Name,
								},
								Annotations: &notionapi.Annotations{
									Code:  true,
									Color: "red",
								},
							}},
							{{Text: &notionapi.Text{Content: childTypeFormatted}}}, // Use corrected type format
							{{Text: &notionapi.Text{Content: formatRequiredField(child.Required)}}},
							{{Text: &notionapi.Text{Content: child.Format}}},      // Display child format if available
							{{Text: &notionapi.Text{Content: child.Description}}}, // Display child description if available
						},
					},
				})
			}
		}
	}

	return rows
}
