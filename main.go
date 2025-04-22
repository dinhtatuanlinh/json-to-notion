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

// Config holds application configuration
type Config struct {
	NotionToken     string
	NotionDBID      string
	InputJSONFiles  []string
	PageTitle       string
}

// Field represents a field in the JSON schema
type Field struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    string      `json:"required"`
	Format      string      `json:"format"`
	Description string      `json:"description"`
	Children    []Field     `json:"children,omitempty"`
	Value       interface{} `json:"value,omitempty"`
}

// NotionService handles Notion API operations
type NotionService struct {
	client *notionapi.Client
	dbID   string
}

// NewNotionService creates a new NotionService instance
func NewNotionService(token, dbID string) *NotionService {
	return &NotionService{
		client: notionapi.NewClient(notionapi.Token(token)),
		dbID:   dbID,
	}
}

// loadConfig loads configuration from flags
func loadConfig(pageTitle string) (*Config, error) {
	config := &Config{
		NotionToken: "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",
		NotionDBID:  "1d863735-e3ae-802e-afd5-d4ed68b646cf",
		PageTitle:   pageTitle,
	}

	return config, nil
}

// formatRequiredField formats the required field indicator
func formatRequiredField(required string) string {
	if required == "true" || required == "Required" {
		return "✅"
	}
	return "❌"
}

// formatType formats the field type for display
func formatType(fieldType string) string {
	typeMap := map[string]string{
		"object":      "obj",
		"array_object": "[]obj",
		"int":         "int",
		"int64":       "int",
		"int32":       "int",
		"uint64":      "int",
		"uint32":      "int",
		"integer":     "int",
		"time.Time":   "datetime",
		"bool":        "boolean",
	}

	if mappedType, ok := typeMap[fieldType]; ok {
		return mappedType
	}

	if strings.HasPrefix(fieldType, "[]") {
		baseType := strings.TrimPrefix(fieldType, "[]")
		if mappedType, ok := typeMap[baseType]; ok {
			return "[]" + mappedType
		}
		return fieldType
	}

	return fieldType
}

// parseFieldData parses field data from JSON
func parseFieldData(name string, fieldData map[string]interface{}) Field {
	field := Field{
		Name:     name,
		Type:     getString(fieldData, "type"),
		Required: fmt.Sprintf("%v", fieldData["required"]),
		Format:   getString(fieldData, "format"),
		Description: getString(fieldData, "description"),
	}

	if items, ok := fieldData["items"].(map[string]interface{}); ok {
		if itemType, ok := items["type"].(string); ok {
			if itemType == "object" || items["properties"] != nil {
				field.Type = "array_object"
				if properties, ok := items["properties"].(map[string]interface{}); ok {
					for propName, propData := range properties {
						if propMap, ok := propData.(map[string]interface{}); ok {
							field.Children = append(field.Children, parseFieldData(propName, propMap))
						}
					}
				}
			} else {
				field.Type = "[]" + itemType
			}
		}
	} else if field.Type == "object" {
		if properties, ok := fieldData["properties"].(map[string]interface{}); ok {
			for propName, propData := range properties {
				if propMap, ok := propData.(map[string]interface{}); ok {
					field.Children = append(field.Children, parseFieldData(propName, propMap))
				}
			}
		}
	}

	return field
}

// getString safely gets a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// createTableRows creates Notion table rows from fields
func createTableRows(fields []Field) []notionapi.Block {
	rows := []notionapi.Block{
		createHeaderRow(),
	}

	for _, field := range fields {
		rows = append(rows, createFieldRow(field))
		for _, child := range field.Children {
			rows = append(rows, createChildRow(child))
		}
	}

	return rows
}

// createHeaderRow creates the table header row
func createHeaderRow() notionapi.Block {
	return &notionapi.TableRowBlock{
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
	}
}

// createFieldRow creates a row for a field
func createFieldRow(field Field) notionapi.Block {
	return &notionapi.TableRowBlock{
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
	}
}

// createChildRow creates a row for a child field
func createChildRow(child Field) notionapi.Block {
	return &notionapi.TableRowBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: "block",
			Type:   "table_row",
		},
		TableRow: notionapi.TableRow{
			Cells: [][]notionapi.RichText{
				{{Text: &notionapi.Text{Content: ""}}},
				{{
					Text: &notionapi.Text{
						Content: child.Name,
					},
					Annotations: &notionapi.Annotations{
						Code:  true,
						Color: "red",
					},
				}},
				{{Text: &notionapi.Text{Content: formatType(child.Type)}}},
				{{Text: &notionapi.Text{Content: formatRequiredField(child.Required)}}},
				{{Text: &notionapi.Text{Content: child.Format}}},
				{{Text: &notionapi.Text{Content: child.Description}}},
			},
		},
	}
}

// createNotionPage creates a Notion page with the provided blocks
func (s *NotionService) createNotionPage(ctx context.Context, title string, blocks []notionapi.Block) (*notionapi.Page, error) {
	pageReq := &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			DatabaseID: notionapi.DatabaseID(s.dbID),
		},
		Properties: notionapi.Properties{
			"Name": notionapi.TitleProperty{
				Title: []notionapi.RichText{
					{
						Text: &notionapi.Text{
							Content: title,
						},
					},
				},
			},
		},
		Children: blocks,
	}

	return s.client.Page.Create(ctx, pageReq)
}

func main() {
	// Define flags
	pageTitle := flag.String("title", "JSON Schema Documentation", "The title for the Notion page")
	flag.Parse()

	config, err := loadConfig(*pageTitle)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Update config with input files
	config.InputJSONFiles = flag.Args()

	notionService := NewNotionService(config.NotionToken, config.NotionDBID)

	// Process each input JSON file
	for _, inputFile := range config.InputJSONFiles {
		jsonData, err := ioutil.ReadFile(inputFile)
		if err != nil {
			log.Printf("Error reading %s: %v", inputFile, err)
			continue
		}

		var data map[string]map[string]interface{}
		if err := json.Unmarshal(jsonData, &data); err != nil {
			log.Printf("Error parsing JSON from %s: %v", inputFile, err)
			continue
		}

		blocks := createBlocks(data)
		page, err := notionService.createNotionPage(context.Background(), config.PageTitle, blocks)
		if err != nil {
			log.Printf("Error creating Notion page for %s: %v", inputFile, err)
			continue
		}

		fmt.Printf("Page created successfully for %s with ID: %s\n", inputFile, page.ID)
	}
}

// createSectionBlocks creates blocks for a section
func createSectionBlocks(section, title string, sectionData map[string]interface{}) []notionapi.Block {
	var blocks []notionapi.Block

	// Add heading
	blocks = append(blocks, &notionapi.Heading1Block{
		BasicBlock: notionapi.BasicBlock{
			Object: "block",
			Type:   "heading_1",
		},
		Heading1: notionapi.Heading{
			RichText: []notionapi.RichText{
				{
					Text: &notionapi.Text{
						Content: title,
					},
				},
			},
		},
	})

	// Add table
	if fields, ok := sectionData["fields"].(map[string]interface{}); ok {
		var sectionFields []Field
		for name, field := range fields {
			if fieldData, ok := field.(map[string]interface{}); ok {
				sectionFields = append(sectionFields, parseFieldData(name, fieldData))
			}
		}

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

	return blocks
}

// createCodeBlock creates a Notion code block with the given content
func createCodeBlock(content string) notionapi.Block {
	return &notionapi.CodeBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: "block",
			Type:   "code",
		},
		Code: notionapi.Code{
			Language: "json",
			RichText: []notionapi.RichText{
				{
					Text: &notionapi.Text{
						Content: content,
					},
				},
			},
		},
	}
}

// createExampleCodeBlock creates a code block with example values
func createExampleCodeBlock(section string, fields map[string]interface{}) notionapi.Block {
	exampleValues := make(map[string]interface{})
	
	for name, field := range fields {
		if fieldData, ok := field.(map[string]interface{}); ok {
			fieldType := getString(fieldData, "type")
			switch fieldType {
			case "uint64":
				exampleValues[name] = 12345
			case "string":
				exampleValues[name] = "example_value"
			case "object":
				// Create example object with all properties
				exampleObject := make(map[string]interface{})
				if properties, ok := fieldData["properties"].(map[string]interface{}); ok {
					for propName, propData := range properties {
						if propMap, ok := propData.(map[string]interface{}); ok {
							propType := getString(propMap, "type")
							switch propType {
							case "string":
								exampleObject[propName] = "example_" + propName
							case "array":
								if items, ok := propMap["items"].(map[string]interface{}); ok {
									if itemType, ok := items["type"].(string); ok {
										if itemType == "string" {
											exampleObject[propName] = []string{"item1", "item2"}
										} else if itemType == "boolean" {
											exampleObject[propName] = []bool{true, false}
										} else if itemType == "integer" || itemType == "int" {
											exampleObject[propName] = []int{1, 2}
										}
									}
								}
							case "boolean":
								exampleObject[propName] = true
							case "integer", "number", "int":
								exampleObject[propName] = 123
							}
						}
					}
				}
				exampleValues[name] = exampleObject
			case "array":
				if items, ok := fieldData["items"].(map[string]interface{}); ok {
					if itemType, ok := items["type"].(string); ok {
						if itemType == "object" {
							// Create example array of objects with all properties
							exampleArray := make([]map[string]interface{}, 1)
							exampleObject := make(map[string]interface{})
							
							if properties, ok := items["properties"].(map[string]interface{}); ok {
								for propName, propData := range properties {
									if propMap, ok := propData.(map[string]interface{}); ok {
										propType := getString(propMap, "type")
										switch propType {
										case "string":
											exampleObject[propName] = "example_" + propName
										case "array":
											if subItems, ok := propMap["items"].(map[string]interface{}); ok {
												if subItemType, ok := subItems["type"].(string); ok {
													if subItemType == "string" {
														exampleObject[propName] = []string{"item1", "item2"}
													} else if subItemType == "boolean" {
														exampleObject[propName] = []bool{true, false}
													} else if subItemType == "integer" || subItemType == "int" {
														exampleObject[propName] = []int{1, 2}
													}
												}
											}
										case "boolean":
											exampleObject[propName] = true
										case "integer", "number", "int":
											exampleObject[propName] = 123
										}
									}
								}
							}
							exampleArray[0] = exampleObject
							exampleValues[name] = exampleArray
						} else {
							exampleValues[name] = []string{"example1", "example2"}
						}
					}
				}
			}
		}
	}

	jsonBytes, err := json.MarshalIndent(exampleValues, "", "    ")
	if err != nil {
		return createCodeBlock("Error creating example: " + err.Error())
	}

	return createCodeBlock(string(jsonBytes))
}

// createBlocks creates Notion blocks from the parsed data
func createBlocks(data map[string]map[string]interface{}) []notionapi.Block {
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

	// Add sections
	sections := []string{"param", "query", "request_body", "response"}
	sectionTitles := map[string]string{
		"param":        "1. Path Parameters",
		"query":        "2. Query Parameters",
		"request_body": "3. Request Body",
		"response":     "4. Response",
	}

	// Process all sections including response
	for _, section := range sections {
		if sectionData, exists := data[section]; exists {
			blocks = append(blocks, createSectionBlocks(section, sectionTitles[section], sectionData)...)
			
			// Add example code block for both request_body and response
			if section == "request_body" || section == "response" {
				if fields, ok := sectionData["fields"].(map[string]interface{}); ok {
					blocks = append(blocks, &notionapi.ParagraphBlock{
						BasicBlock: notionapi.BasicBlock{
							Object: "block",
							Type:   "paragraph",
						},
						Paragraph: notionapi.Paragraph{
							RichText: []notionapi.RichText{
								{
									Text: &notionapi.Text{
										Content: fmt.Sprintf("Example %s:", strings.Title(strings.Replace(section, "_", " ", -1))),
									},
								},
							},
						},
					})
					blocks = append(blocks, createExampleCodeBlock(section, fields))
				}
			}
		}
	}

	return blocks
}
